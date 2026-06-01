package dbcache

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"time"

	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/store"
	"github.com/gomooth/pkg/framework/dbquery"
	"github.com/gomooth/pkg/framework/telemetry"
	pkgxcode "github.com/gomooth/pkg/framework/xcode"

	"github.com/redis/go-redis/v9"

	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/sync/singleflight"
)

type dbCache[E, F any] struct {
	cacheManager   *cache.Cache[string]
	name           string
	autoRenew      bool // 自动延长缓存有效期
	expiration     time.Duration
	renewThreshold float64       // 续期阈值比例
	codec          Codec         // 序列化编解码器
	errorCacheTTL  time.Duration // 错误结果缓存时间，0 表示不缓存错误
	single         singleflight.Group  // 按 name 前缀隔离，不同 dbCache 实例不会碰撞
	renewSingle    singleflight.Group // 续期去重，防止并发续期风暴
}

// errorCacheKeySuffix 错误占位值的缓存键后缀，与正常数据完全隔离
const errorCacheKeySuffix = ":__err__"

var dbCacheMeter = telemetry.Meter("dbcache")

var (
	dbCacheHitCounter, _           = dbCacheMeter.Int64Counter("dbcache.hit")
	dbCacheMissCounter, _          = dbCacheMeter.Int64Counter("dbcache.miss")
	dbCacheRenewCounter, _         = dbCacheMeter.Int64Counter("dbcache.renew")
	dbCacheErrorCacheHitCounter, _ = dbCacheMeter.Int64Counter("dbcache.error_cache.hit")
	dbCacheWriteCounter, _         = dbCacheMeter.Int64Counter("dbcache.write")
	dbCacheOperationDuration, _    = dbCacheMeter.Float64Histogram("dbcache.operation.duration",
		metric.WithUnit("s"))
)

func recordDBCacheDuration(ctx context.Context, namespace, operation string, dur time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	attrs := metric.WithAttributes(
		attribute.String("namespace", namespace),
		attribute.String("operation", operation),
		attribute.String("result", result),
	)
	dbCacheOperationDuration.Record(ctx, dur.Seconds(), attrs)
}

// hashKey 使用 FNV-1a 128位哈希生成缓存键，比 MD5 更快且足够用于非密码学场景
func hashKey(s string) string {
	h := fnv.New128a()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// New 创建数据库缓存实例。
//
// 默认使用 JSON 编解码器，可通过 WithCodec 选项替换为 msgpack 或 gob 等更高效的实现。
// 注意：更换编解码器会使现有缓存数据失效。
func New[E, F any](name string, cacheManager *cache.Cache[string], opts ...func(*option)) IDBCache[E, F] {
	cnf := &option{
		autoRenew:      true,
		expiration:     5 * time.Minute,
		renewThreshold: 0.2,
		codec:          JSONCodec{},
		errorCacheTTL:  0, // 默认不缓存错误
	}
	for _, opt := range opts {
		opt(cnf)
	}

	return &dbCache[E, F]{
		name:           name,
		cacheManager:   cacheManager,
		autoRenew:      cnf.autoRenew,
		expiration:     cnf.expiration,
		renewThreshold: cnf.renewThreshold,
		codec:          cnf.codec,
		errorCacheTTL:  cnf.errorCacheTTL,
	}
}

type queryResult[E any] struct {
	Paginate struct {
		Data  []*E `json:"data"`
		Total uint `json:"total"`
	} `json:"paginate,omitempty"`

	List struct {
		Data []*E `json:"data"`
	} `json:"list,omitempty"`

	First struct {
		Data *E `json:"data"`
	} `json:"first,omitempty"`
}

func (s *dbCache[E, F]) Codec() Codec {
	return s.codec
}

// cacheQuery 封装 "填充 queryResult → marshal → remember → unmarshal" 流程
func (s *dbCache[E, F]) cacheQuery(
	ctx context.Context, key string, tags []string,
	fill func(ctx context.Context) (*queryResult[E], error),
) (*queryResult[E], error) {
	cacheData, err := s.remember(ctx, key, tags, func(ctx context.Context) ([]byte, error) {
		res, err := fill(ctx)
		if err != nil {
			return nil, err
		}

		return s.codec.Marshal(res)
	})
	if err != nil {
		return nil, err
	}

	var result *queryResult[E]
	if err := s.codec.Unmarshal(cacheData, &result); err != nil {
		return nil, xerror.WrapWithXCode(err, pkgxcode.ErrCacheReadFailed)
	}
	return result, nil
}

func (s *dbCache[E, F]) Paginate(ctx context.Context, q dbquery.IQuery[F],
	query func(ctx context.Context) ([]*E, uint, error),
) (records []*E, total uint, err error) {
	start := time.Now()
	defer func() {
		recordDBCacheDuration(ctx, s.name, "paginate", time.Since(start), err)
	}()
	k := hashKey(q.String())
	offset, limit, _ := dbquery.PaginateValues(q)
	key := dbquery.FormatPaginateKey(s.name, offset, limit, k)
	tags := []string{s.tag("paginate")}

	result, err := s.cacheQuery(ctx, key, tags, func(ctx context.Context) (*queryResult[E], error) {
		records, total, err := query(ctx)
		if err != nil {
			return nil, err
		}

		res := new(queryResult[E])
		res.Paginate.Data = records
		res.Paginate.Total = total
		return res, nil
	})
	if err != nil {
		return nil, 0, err
	}

	return result.Paginate.Data, result.Paginate.Total, nil
}

func (s *dbCache[E, F]) List(ctx context.Context, q dbquery.IQuery[F],
	query func(ctx context.Context) ([]*E, error),
) (records []*E, err error) {
	start := time.Now()
	defer func() {
		recordDBCacheDuration(ctx, s.name, "list", time.Since(start), err)
	}()
	k := hashKey(q.String())
	key := dbquery.FormatListKey(s.name, k)
	tags := []string{s.tag("list")}

	result, err := s.cacheQuery(ctx, key, tags, func(ctx context.Context) (*queryResult[E], error) {
		records, err := query(ctx)
		if err != nil {
			return nil, err
		}

		res := new(queryResult[E])
		res.List.Data = records
		return res, nil
	})
	if err != nil {
		return nil, err
	}

	return result.List.Data, nil
}

func (s *dbCache[E, F]) First(ctx context.Context, id uint, query func(ctx context.Context) (*E, error)) (record *E, err error) {
	start := time.Now()
	defer func() {
		recordDBCacheDuration(ctx, s.name, "first", time.Since(start), err)
	}()
	if id == 0 {
		return nil, xerror.NewXCode(xcode.RequestParamError, "id error")
	}

	tags := []string{s.tag(fmt.Sprintf("%d", id))}
	key := fmt.Sprintf("%s:first:%d", s.name, id)

	result, err := s.cacheQuery(ctx, key, tags, func(ctx context.Context) (*queryResult[E], error) {
		record, err := query(ctx)
		if err != nil {
			return nil, err
		}

		res := new(queryResult[E])
		res.First.Data = record
		return res, nil
	})
	if err != nil {
		return nil, err
	}

	return result.First.Data, nil
}

func (s *dbCache[E, F]) Clear(ctx context.Context, opts ...func(*clearOption)) (err error) {
	start := time.Now()
	defer func() {
		recordDBCacheDuration(ctx, s.name, "clear", time.Since(start), err)
	}()
	cnf := new(clearOption)
	for _, opt := range opts {
		opt(cnf)
	}

	if s.cacheManager == nil {
		return xerror.NewXCode(pkgxcode.ErrCacheNotInitialized, "dbcache: manager not initialized")
	}

	// 显式指定清理所有缓存
	if cnf.all {
		return s.cacheManager.Invalidate(ctx, store.WithInvalidateTags([]string{
			s.ownTag(),
		}))
	}
	// 未指定任何选项时返回错误，避免静默无操作导致调用方误以为缓存已清除
	if !cnf.single && !cnf.all {
		return xerror.NewXCode(xcode.RequestParamError, "dbcache: Clear requires at least one option (e.g. ClearWithAll, ClearWithID)")
	}

	tags := make([]string, 0)
	if len(cnf.ids) > 0 {
		for _, id := range cnf.ids {
			tags = append(tags, s.tag(fmt.Sprintf("%d", id)))
		}
	}
	if len(cnf.keys) > 0 {
		for _, key := range cnf.keys {
			tags = append(tags, s.tag(key))
		}
	}
	if len(cnf.tags) > 0 {
		tags = append(tags, cnf.tags...)
	}

	if cnf.paginate {
		tags = append(tags, s.tag("paginate"))
	}
	if cnf.list {
		tags = append(tags, s.tag("list"))
	}
	if cnf.remember {
		tags = append(tags, s.tag("remember"))
	}

	if len(tags) > 0 {
		return s.cacheManager.Invalidate(ctx, store.WithInvalidateTags(tags))
	}
	return nil
}

func (s *dbCache[E, F]) Remember(ctx context.Context, key string, query func(ctx context.Context) ([]byte, error)) (result []byte, err error) {
	start := time.Now()
	defer func() {
		recordDBCacheDuration(ctx, s.name, "remember", time.Since(start), err)
	}()
	tags := []string{
		s.tag(key),
		s.tag("remember"),
		s.tag(fmt.Sprintf("remember:%s", key)),
	}
	cacheKey := fmt.Sprintf("%s:remember:%s", s.name, key)

	return s.remember(ctx, cacheKey, tags, query)
}

// remember 核心缓存逻辑：查缓存 → 命中则续期 → 未命中则 singleflight 执行 fn 并写入缓存
// 内部以 []byte 流转数据，仅在缓存存储边界做 string 转换，避免 Remember 方法的双重拷贝
func (s *dbCache[E, F]) remember(ctx context.Context, key string, tags []string,
	fun func(ctx context.Context) ([]byte, error),
) ([]byte, error) {
	if s.cacheManager == nil {
		return nil, xerror.NewXCode(pkgxcode.ErrCacheNotInitialized, "dbcache: manager not initialized")
	}

	if err := ctx.Err(); err != nil {
		return nil, xerror.WrapWithXCode(err, pkgxcode.ErrCacheReadFailed)
	}

	cachedTags := append([]string{"dbcache", s.ownTag()}, tags...)
	cacheData, d, err := s.cacheManager.GetWithTTL(ctx, key)
	if err == nil {
		dbCacheHitCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("namespace", s.name)))

		// 延长时效：当剩余 TTL <= expiration * renewThreshold 时续期
		// 使用 renewSingle 去重，避免多个 goroutine 同时续期同一 key 造成写入风暴
		if s.autoRenew && d <= time.Duration(float64(s.expiration)*s.renewThreshold) {
			if _, err, _ := s.renewSingle.Do(key, func() (any, error) {
				if err := s.cacheManager.Set(
					ctx, key, cacheData,
					store.WithExpiration(s.expiration),
					store.WithTags(cachedTags),
				); err != nil {
					slog.Warn("dbcache: auto-renew set failed", slog.String("component", "dbcache"), slog.String("key", key), slog.String("error", err.Error()))
					return nil, err
				}
				dbCacheRenewCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("namespace", s.name), attribute.String("result", "success")))
				return nil, nil
			}); err != nil {
				// 续期失败不影响本次读取，仅记录日志
				slog.Warn("dbcache: auto-renew failed", slog.String("component", "dbcache"), slog.String("key", key), slog.String("error", err.Error()))
				dbCacheRenewCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("namespace", s.name), attribute.String("result", "failure")))
			}
		}

		return []byte(cacheData), nil
	}

	// Graceful degradation: 任何 Get 错误都视为缓存未命中，继续查 DB
	// 对非 redis.Nil 的异常错误（如网络断连）记录日志，方便排查
	if !errors.Is(err, redis.Nil) {
		slog.Debug("dbcache: cache miss with unexpected error, falling back to query",
			slog.String("component", "dbcache"), slog.String("key", key), slog.String("error", err.Error()))
	}

	dbCacheMissCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("namespace", s.name)))

	// 检查是否有错误占位值（独立键存储，与正常数据完全隔离）
	if s.errorCacheTTL > 0 {
		errKey := key + errorCacheKeySuffix
		if errData, _, errErr := s.cacheManager.GetWithTTL(ctx, errKey); errErr == nil {
			dbCacheErrorCacheHitCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("namespace", s.name)))
			return nil, xerror.NewXCode(pkgxcode.ErrCacheMiss, errData)
		}
	}

	v, err, _ := s.single.Do(key, func() (any, error) {
		result, err := fun(ctx)
		if err != nil {
			// 错误结果短暂缓存到独立键：防止相同 key 的错误请求反复打到数据库
			if s.errorCacheTTL > 0 {
				errKey := key + errorCacheKeySuffix
				if setErr := s.cacheManager.Set(
					ctx, errKey, err.Error(),
					store.WithExpiration(s.errorCacheTTL),
					store.WithTags(cachedTags),
				); setErr != nil {
					slog.Debug("dbcache: error cache set failed", slog.String("component", "dbcache"), slog.String("key", errKey), slog.String("error", setErr.Error()))
				} else {
					dbCacheWriteCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("namespace", s.name), attribute.String("result", "failure")))
				}
			}
			return nil, err
		}

		// string(result) 会产生一次 []byte→string 拷贝，这是 gocache Cache[string] 的 API 限制。
		// 大 payload 场景若需避免此开销，建议直接使用底层 gocache 实例。
		if err := s.cacheManager.Set(
			ctx, key, string(result),
			store.WithExpiration(s.expiration),
			store.WithTags(cachedTags),
		); err != nil {
			slog.Error("dbcache: cache set failed, degrading to direct result",
				slog.String("component", "dbcache"), slog.String("key", key), slog.String("error", err.Error()))
			dbCacheWriteCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("namespace", s.name), attribute.String("result", "failure")))
		} else {
			dbCacheWriteCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("namespace", s.name), attribute.String("result", "success")))
		}

		return result, nil
	})
	if err != nil {
		return nil, err
	}

	result, ok := v.([]byte)
	if !ok {
		return nil, xerror.NewXCode(pkgxcode.ErrCacheReadFailed, "cache store result type assertion failed")
	}
	return result, nil
}

func (s *dbCache[E, F]) ownTag() string {
	return fmt.Sprintf("dbcache:%s", s.name)
}

func (s *dbCache[E, F]) tag(tag string) string {
	return fmt.Sprintf("dbcache:%s:%s", s.name, tag)
}

func (s *dbCache[E, F]) Forget(ctx context.Context, key string) (err error) {
	start := time.Now()
	defer func() {
		recordDBCacheDuration(ctx, s.name, "forget", time.Since(start), err)
	}()
	if s.cacheManager == nil {
		return xerror.NewXCode(pkgxcode.ErrCacheNotInitialized, "dbcache: manager not initialized")
	}

	tags := []string{
		s.tag(key),
		s.tag(fmt.Sprintf("remember:%s", key)),
	}
	return s.cacheManager.Invalidate(ctx, store.WithInvalidateTags(tags))
}
