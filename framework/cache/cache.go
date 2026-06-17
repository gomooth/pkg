package cache

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/gomooth/pkg/framework/telemetry"
	pkgxcode "github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/utils/strutil"

	"github.com/gomooth/xerror"
)

var (
	cacheHitCounter   metric.Int64Counter
	cacheMissCounter  metric.Int64Counter
	cacheSetCounter   metric.Int64Counter
	cacheEvictCounter metric.Int64Counter
)

// NeverExpire 永久缓存标记值，设置此值时缓存不会过期
const NeverExpire time.Duration = -1

var defaultExpireNs atomic.Int64

func init() {
	defaultExpireNs.Store(int64(5 * time.Minute))

	telemetry.OnProviderSet(func() {
		m := telemetry.Meter("cache")
		cacheHitCounter, _ = m.Int64Counter("cache.core.hit")
		cacheMissCounter, _ = m.Int64Counter("cache.core.miss")
		cacheSetCounter, _ = m.Int64Counter("cache.core.set")
		cacheEvictCounter, _ = m.Int64Counter("cache.core.evict")
	})
}

// startCacheMethodSpan 创建 cache 方法级 OTel Span
func startCacheMethodSpan[T any](ctx context.Context, c *anyCache[T], operation string) (context.Context, trace.Span) {
	if c.traceConfig == nil || !c.traceConfig.methodSpan {
		return ctx, nil
	}
	ctx, span := c.tracer.Start(ctx, "cache."+operation,
		trace.WithAttributes(
			attribute.String("cache.operation", operation),
			attribute.String("cache.namespace", c.name),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	return ctx, span
}

// finishSpan 完成 Span，记录错误（如有）
func finishSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

func getDefaultExpire() time.Duration {
	return time.Duration(defaultExpireNs.Load())
}

// Option 缓存配置选项
type Option[T any] func(*cacheOption[T])

// WithMaxItems 设置缓存最大条目数，超过后新 key 的 Set 将返回错误（已有 key 更新不受限）
// 需配合 WithItemCountFunc 使用以提供条目计数能力
func WithMaxItems[T any](n int) Option[T] {
	return func(o *cacheOption[T]) {
		if n > 0 {
			o.maxItems = n
		}
	}
}

// WithItemCountFunc 设置获取当前缓存条目数的函数
// 通常传入底层 go-cache 实例的 ItemCount 方法
func WithItemCountFunc[T any](fn func() int) Option[T] {
	return func(o *cacheOption[T]) {
		o.itemCountFunc = fn
	}
}

// WithAutoRenew 设置是否自动续期，默认为 true。
// 当缓存命中且剩余 TTL 低于阈值时，自动延长缓存有效期，防止热点 key 过期瞬间的缓存击穿。
func WithAutoRenew[T any](enabled bool) Option[T] {
	return func(o *cacheOption[T]) {
		o.autoRenew = enabled
	}
}

// WithRenewThreshold 设置自动续期阈值比例，默认 0.2。
// 当剩余 TTL <= expire * threshold 时触发续期。仅在 autoRenew 为 true 时生效。
func WithRenewThreshold[T any](threshold float64) Option[T] {
	return func(o *cacheOption[T]) {
		if threshold > 0 && threshold < 1 {
			o.renewThreshold = threshold
		}
	}
}

// cacheOption 缓存配置选项的中间结构体
type cacheOption[T any] struct {
	maxItems       int
	itemCountFunc  func() int
	autoRenew      bool
	renewThreshold float64
	traceConfig    *traceConfig // OTel Span 配置
}

// traceConfig OTel Span 配置
type traceConfig struct {
	methodSpan bool // 方法级 Span，默认 true
	buildSpan  bool // 构建级 Span，默认 false
}

// WithCacheTraceMethodSpan 开启方法级 OTel Span（默认已开启）
func WithCacheTraceMethodSpan[T any]() Option[T] {
	return func(o *cacheOption[T]) {
		if o.traceConfig == nil {
			o.traceConfig = &traceConfig{methodSpan: true, buildSpan: false}
		}
		o.traceConfig.methodSpan = true
	}
}

// WithCacheTraceBuildSpan 开启构建级 OTel Span（隐含同时开启方法级 Span）
func WithCacheTraceBuildSpan[T any]() Option[T] {
	return func(o *cacheOption[T]) {
		if o.traceConfig == nil {
			o.traceConfig = &traceConfig{methodSpan: true, buildSpan: true}
		}
		o.traceConfig.buildSpan = true
	}
}

type anyCache[T any] struct {
	cacheManager   *cache.Cache[T]
	name           string
	single         singleflight.Group
	renewSingle    singleflight.Group
	maxItems       int
	itemCountFunc  func() int
	autoRenew      bool
	renewThreshold float64
	tracer         trace.Tracer
	modelName      string
	traceConfig    *traceConfig
}

var _ ICache[any] = (*anyCache[any])(nil)

// New 创建缓存实例，nameSpace 为命名空间用于 key 前缀隔离，cacheManager 为底层 gocache 管理器
func New[T any](nameSpace string, cacheManager *cache.Cache[T], opts ...Option[T]) ICache[T] {
	cnf := &cacheOption[T]{
		autoRenew:      true,
		renewThreshold: 0.2,
	}

	for _, opt := range opts {
		opt(cnf)
	}

	tc := cnf.traceConfig
	if tc == nil {
		tc = &traceConfig{methodSpan: true, buildSpan: false}
	}

	return &anyCache[T]{
		name:           nameSpace,
		cacheManager:   cacheManager,
		autoRenew:      cnf.autoRenew,
		renewThreshold: cnf.renewThreshold,
		maxItems:       cnf.maxItems,
		itemCountFunc:  cnf.itemCountFunc,
		tracer:         telemetry.Tracer("cache"),
		modelName:      reflect.TypeOf(new(T)).Elem().Name(),
		traceConfig:    tc,
	}
}

func (c *anyCache[T]) getKey(key string) string {
	return fmt.Sprintf("%s:%s", strutil.Camel(c.name), key)
}

func (c *anyCache[T]) Get(ctx context.Context, key string) (_ *T, _ time.Duration, err error) {
	ctx, span := startCacheMethodSpan[T](ctx, c, "get")
	defer func() {
		finishSpan(span, err)
	}()

	if c.cacheManager == nil {
		return nil, 0, xerror.NewXCode(pkgxcode.ErrCacheNotInitialized, "cache: manager not initialized")
	}

	key = c.getKey(key)
	cacheData, d, err := c.cacheManager.GetWithTTL(ctx, key)
	if err == nil {
		cacheHitCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("namespace", c.name),
		))
		return &cacheData, d, nil
	}

	cacheMissCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("namespace", c.name),
	))
	return nil, 0, err
}

func (c *anyCache[T]) GetAndDelete(ctx context.Context, key string) (_ *T, err error) {
	ctx, span := startCacheMethodSpan[T](ctx, c, "get_and_delete")
	defer func() {
		finishSpan(span, err)
	}()

	if c.cacheManager == nil {
		return nil, xerror.NewXCode(pkgxcode.ErrCacheNotInitialized, "cache: manager not initialized")
	}

	key = c.getKey(key)
	cacheData, err := c.cacheManager.Get(ctx, key)
	if err == nil {
		if err := c.cacheManager.Delete(ctx, key); err != nil {
			return nil, err
		}

		return &cacheData, nil
	}

	return nil, err
}

func (c *anyCache[T]) Set(ctx context.Context, key string, val *T, expire time.Duration) (err error) {
	ctx, span := startCacheMethodSpan[T](ctx, c, "set")
	defer func() {
		finishSpan(span, err)
	}()

	if c.cacheManager == nil {
		return xerror.NewXCode(pkgxcode.ErrCacheNotInitialized, "cache: manager not initialized")
	}

	// 永久缓存：跳过默认过期时间的替换
	// gocache store.WithExpiration(0) 表示不过期
	var expireOpt store.Option
	if expire == NeverExpire {
		expireOpt = store.WithExpiration(0)
	} else if expire == 0 {
		// 未设置过期时间时使用默认过期时间
		expire = getDefaultExpire()
		expireOpt = store.WithExpiration(expire)
	} else {
		expireOpt = store.WithExpiration(expire)
	}

	key = c.getKey(key)

	// 容量检查：超限时仅允许已有 key 更新，拒绝新 key。
	// 注意：Get 和 itemCountFunc 之间存在 TOCTOU 窗口，并发写入可能导致实际条目数短暂超过 maxItems。
	// 这是容量软限制（防止缓存无限增长），不保证严格精确计数，额外的锁保护会引入性能开销。
	if c.maxItems > 0 && c.itemCountFunc != nil {
		_, getErr := c.cacheManager.Get(ctx, key)
		if getErr != nil {
			if c.itemCountFunc() >= c.maxItems {
				cacheEvictCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("namespace", c.name),
				))
				return xerror.New("cache: capacity limit reached")
			}
		}
	}

	cacheSetCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("namespace", c.name),
	))
	return c.cacheManager.Set(ctx, key, *val, expireOpt)
}

func (c *anyCache[T]) Remember(
	ctx context.Context,
	key string,
	expire time.Duration,
	fun func(ctx context.Context) (*T, error),
) (_ *T, err error) {
	ctx, span := startCacheMethodSpan[T](ctx, c, "remember")
	defer func() {
		finishSpan(span, err)
	}()

	if c.cacheManager == nil {
		return nil, xerror.NewXCode(pkgxcode.ErrCacheNotInitialized, "cache: manager not initialized")
	}

	cacheKey := c.getKey(key)
	cd, ttl, err := c.cacheManager.GetWithTTL(ctx, cacheKey)
	if err == nil {
		// 自动续期：当剩余 TTL 低于阈值时延长有效期
		if c.autoRenew && ttl > 0 && expire != NeverExpire {
			renewExpire := expire
			if renewExpire == 0 {
				renewExpire = getDefaultExpire()
			}
			if renewExpire > 0 && ttl <= time.Duration(float64(renewExpire)*c.renewThreshold) {
				if _, renewErr, _ := c.renewSingle.Do(cacheKey, func() (any, error) {
					if setErr := c.cacheManager.Set(ctx, cacheKey, cd, store.WithExpiration(renewExpire)); setErr != nil {
						slog.Warn("cache: auto-renew set failed", slog.String("component", "cache"), slog.String("key", cacheKey), slog.String("error", setErr.Error()))
						return nil, setErr
					}
					return nil, nil
				}); renewErr != nil {
					slog.Warn("cache: auto-renew failed", slog.String("component", "cache"), slog.String("key", cacheKey), slog.String("error", renewErr.Error()))
				}
			}
		}
		return &cd, nil
	}

	v, err, _ := c.single.Do(cacheKey, func() (any, error) {
		data, err := fun(ctx)
		if err != nil {
			return nil, err
		}

		var rememberExpireOpt store.Option
		if expire == NeverExpire {
			rememberExpireOpt = store.WithExpiration(0)
		} else {
			rememberExpireOpt = store.WithExpiration(expire)
		}

		if setErr := c.cacheManager.Set(ctx, cacheKey, *data, rememberExpireOpt); setErr != nil {
			slog.Error("cache: set failed, degrading to direct result", slog.String("component", "cache"), slog.String("key", cacheKey), slog.String("error", setErr.Error()))
		}

		return data, nil
	})
	if err != nil {
		return nil, err
	}

	data, ok := v.(*T)
	if !ok {
		return nil, xerror.New("cache remember result type assertion failed")
	}

	return data, nil
}

func (c *anyCache[T]) Clear(ctx context.Context, key string) (err error) {
	ctx, span := startCacheMethodSpan[T](ctx, c, "clear")
	defer func() {
		finishSpan(span, err)
	}()

	if c.cacheManager == nil {
		return xerror.NewXCode(pkgxcode.ErrCacheNotInitialized, "cache: manager not initialized")
	}

	key = c.getKey(key)
	return c.cacheManager.Delete(ctx, key)
}

// SetDefaultExpire 设置默认过期时间
func SetDefaultExpire(d time.Duration) {
	defaultExpireNs.Store(int64(d))
}
