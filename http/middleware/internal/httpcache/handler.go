package httpcache

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/framework/telemetry"
	"github.com/gomooth/pkg/http/middleware/internal/httpcache/store"
	"github.com/gomooth/xerror"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var httpCacheMeter = telemetry.Meter("httpcache")

var (
	httpCacheHitCounter, _    = httpCacheMeter.Int64Counter("httpcache.hit")
	httpCacheMissCounter, _   = httpCacheMeter.Int64Counter("httpcache.miss")
	httpCacheWriteCounter, _  = httpCacheMeter.Int64Counter("httpcache.write")
	httpCacheErrorCounter, _  = httpCacheMeter.Int64Counter("httpcache.error")
)

type handler struct {
	debug                 bool
	singleFlightTimeout   time.Duration
	withoutResponseHeader bool
	prefixKey             string
	log                   *slog.Logger

	store      store.ICacheStore
	userIDFunc func(*gin.Context) (uint, error) // 从请求上下文提取用户 ID，替代直接依赖 jwt

	globalCacheDuration    time.Duration
	globalHeaderKeys       []string            // 用于计算缓存的 header
	globalSkipFields       map[string]struct{} // 不用于计算缓存的 key
	globalRequestHeaderKey map[string]struct{}

	routeList     []string             // 路由规则排序列表
	routePolicies map[string]*ruleItem // 路由特殊规则: urlPathRegrex => ruleItem
}

func New(opts ...Option) gin.HandlerFunc {
	h, _ := NewWithCloser(opts...)
	return h
}

// NewWithCloser 创建 httpcache 中间件，同时返回一个关闭函数。
// 关闭函数会释放 store 持有的资源（如内部创建的 Redis 连接），应在应用关闭时调用。
func NewWithCloser(opts ...Option) (gin.HandlerFunc, func() error) {
	f := &handler{
		globalCacheDuration: 5 * time.Minute,
		globalHeaderKeys:    make([]string, 0),
		globalSkipFields:    make(map[string]struct{}, 0),
		routeList:           make([]string, 0),
		routePolicies:       make(map[string]*ruleItem, 0),
		singleFlightTimeout: 10 * time.Millisecond, // 100QPS
	}

	for _, opt := range opts {
		opt(f)
	}

	handlerFunc := func(c *gin.Context) {
		strategy, err := f.getCacheStrategy(c)
		if err != nil {
			slog.Error("get http cache strategy failed", slog.String("component", "httpcache"), slog.String("error", err.Error()))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": "internal cache error",
			})
			return
		}

		if f.store == nil {
			c.Next()
			return
		}

		if !strategy.NeedCached {
			c.Next()
			return
		}

		cached, respCache, err := f.cached(c, strategy)
		if err != nil {
			slog.Error("http cache handle failed", slog.String("component", "httpcache"), slog.String("error", err.Error()))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": "internal cache error",
			})
			return
		}

		if !cached {
			c.Next()
			return
		}

		c.Writer.WriteHeader(respCache.Status)

		if !f.withoutResponseHeader {
			for key, values := range respCache.Header {
				for _, val := range values {
					c.Writer.Header().Set(key, val)
				}
			}
		}

		if _, err := c.Writer.Write(respCache.Data); err != nil {
			slog.Error("http cache write response failed", slog.String("component", "httpcache"), slog.String("error", err.Error()))
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": "internal cache error",
			})
			return
		}

		// 跳出，不走后续的中间件
		c.Abort()
	}

	closeFn := func() error {
		if closer, ok := f.store.(io.Closer); ok {
			return closer.Close()
		}
		return nil
	}

	return handlerFunc, closeFn
}

func (h *handler) getCacheStrategy(ctx *gin.Context) (*strategy, error) {
	fullPath := ctx.FullPath()
	method := ctx.Request.Method

	// 只缓存 get/delete 成功的请求
	if method != http.MethodGet && method != http.MethodDelete {
		h.debugf("http method is not GET/DELETE, skip. url=[%s]%s", method, fullPath)
		return &strategy{
			NeedCached: false,
		}, nil
	}

	// 获取路由单独的缓存策略，如果存在多个，则以最后一个为准
	var rule *ruleItem
	for _, route := range h.routeList {
		if strings.Contains(fullPath, route) {
			rule = h.routePolicies[route]
		}
	}

	if rule == nil {
		h.debugf("not hit strategy, skip. url=%s", fullPath)
		return &strategy{
			NeedCached: false,
		}, nil
	}
	h.debugf("found cache strategy: rule=%s, url=%s", rule.String(), fullPath)

	qs := ctx.Request.URL.Query()
	params := url.Values{}
	for key := range qs {
		if len(rule.fields) == 0 {
			_, gok := h.globalSkipFields[key]
			_, ok := rule.skipFields[key]
			if !gok && !ok {
				params.Add(key, qs.Get(key))
			}
		} else {
			if _, ok := rule.fields[key]; !ok {
				params.Add(key, qs.Get(key))
			}
		}
	}

	headers := ctx.Request.Header
	headerKeys := append(h.globalHeaderKeys, rule.headerKeys...)
	for _, key := range headerKeys {
		params.Add(strings.ToLower(key), headers.Get(key))
	}

	var userID uint
	if rule.withToken {
		if h.userIDFunc == nil {
			h.debugf("withToken enabled but userIDFunc not set")
			return nil, xerror.New("httpcache: withToken requires userIDFunc")
		}
		uid, err := h.userIDFunc(ctx)
		if err != nil {
			h.debugf("get user id failed, err=%+v", err)
			return nil, xerror.Wrap(err, "get user id failed")
		}
		userID = uid
	}

	cacheKey := ctx.Request.URL.Path + ":" + params.Encode()
	if userID > 0 {
		cacheKey += ":uid=" + strconv.Itoa(int(userID))
	}
	h.debugf("get cache strategy input: qs=%s, key=%s", qs.Encode(), cacheKey)

	duration := h.globalCacheDuration
	if rule.duration > 0 {
		duration = rule.duration
	}

	return &strategy{
		NeedCached:    true,
		CacheKey:      cacheKey,
		CacheDuration: duration,
	}, nil
}

func (h *handler) debugf(format string, vals ...interface{}) {
	if !h.debug {
		return
	}

	if h.log != nil {
		h.log.Debug(fmt.Sprintf("[httpcache] "+format, vals...), slog.String("component", "httpcache"))
		return
	}

	slog.Debug(fmt.Sprintf("[httpcache] "+format, vals...), slog.String("component", "httpcache"))
}

func (h *handler) cached(c *gin.Context, strategy *strategy) (bool, *store.CachedResponse, error) {
	cacheKey := h.getCacheKey(strategy.CacheKey)

	data, err, _ := sf.Do(cacheKey, func() (any, error) {
		// 限制 QPS = 1s/h.singleFlightTimeout
		if h.singleFlightTimeout > 0 {
			timer := time.AfterFunc(h.singleFlightTimeout, func() {
				sf.Forget(cacheKey)
			})
			defer timer.Stop()
		}

		// 先获取缓存
		respCache := store.CachedResponse{}
		err := h.store.Get(c.Request.Context(), cacheKey, &respCache)
		if err == nil {
			h.debugf("hit cache, key=%s", cacheKey)
			httpCacheHitCounter.Add(c.Request.Context(), 1)
			return &respCache, nil
		}

		if err != store.ErrorCacheMiss {
			httpCacheErrorCounter.Add(c.Request.Context(), 1, metric.WithAttributes(attribute.String("phase", "get")))
			return nil, xerror.Wrapf(err, "get http cache failed, key=%s", cacheKey)
		}

		httpCacheMissCounter.Add(c.Request.Context(), 1)

		// 未获取到缓存，调用下一个请求链
		// 将自定义的响应写入器传递给 Gin 的下一个处理器，便于复制和缓存 response
		cacheWriter := newResponseWriter(c.Writer)
		c.Writer = cacheWriter
		c.Next()

		// 非成功请求，不进行缓存
		if c.Writer.Status() < 200 || c.Writer.Status() >= 300 {
			h.debugf("http request not success, skip. statusCode=%d", c.Writer.Status())
			return nil, nil
		}

		// 保存缓存
		resp := newCachedResponse(cacheWriter)
		if err := h.store.Set(c.Request.Context(), cacheKey, resp, strategy.CacheDuration); err != nil {
			httpCacheErrorCounter.Add(c.Request.Context(), 1, metric.WithAttributes(attribute.String("phase", "set")))
			httpCacheWriteCounter.Add(c.Request.Context(), 1, metric.WithAttributes(attribute.String("result", "failure")))
			return nil, xerror.Wrapf(err, "set http cache failed, key=%s", cacheKey)
		}
		httpCacheWriteCounter.Add(c.Request.Context(), 1, metric.WithAttributes(attribute.String("result", "success")))

		// 从请求链中获取的数据，直接跳过。防止响应重复
		h.debugf("not cache, save cache and redirect next, key=%s", cacheKey)
		return nil, nil
	})

	if err != nil {
		// 非 debug 模式，不阻塞
		if !h.debug {
			return false, nil, nil
		}
		return false, nil, err
	}

	// 不需要缓存
	if data == nil {
		return false, nil, nil
	}

	return true, data.(*store.CachedResponse), nil
}

func (h *handler) getCacheKey(key string) string {
	var bf bytes.Buffer
	bf.WriteString("httpCache:")
	if len(h.prefixKey) > 0 {
		bf.WriteString(h.prefixKey)
		bf.WriteString(":")
	}
	bf.WriteString(key)
	return bf.String()
}

func (h *handler) replyWithCache(c *gin.Context, respCache *store.CachedResponse) {
	c.Writer.WriteHeader(respCache.Status)

	if !h.withoutResponseHeader {
		for key, values := range respCache.Header {
			for _, val := range values {
				c.Writer.Header().Set(key, val)
			}
		}
	}

	if _, err := c.Writer.Write(respCache.Data); err != nil {
		h.debugf("write response error: %s", err)
	}
}
