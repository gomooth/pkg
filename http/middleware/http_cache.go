package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gomooth/pkg/http/jwt"
	"github.com/gomooth/pkg/http/middleware/internal/httpcache"

	"github.com/redis/go-redis/v9"

	"github.com/save95/xlog"
)

// HttpCache http 响应缓存
//
// usage:
//
//	r.Use(middleware.HttpCache(
//		middleware.WithHttpCacheDebug(),
//		middleware.WithHttpCacheLogger(global.Log),
//		middleware.WithHttpCacheJWTOption(global.JWTOption(false)),
//		middleware.WithHttpCacheGlobalDuration(5*time.Minute),
//		middleware.WithHttpCacheRedisStore(redis.NewClient(&redis.Options{
//			Addr:     global.Config.HttpCache.Addr,
//			Password: global.Config.HttpCache.Password,
//			DB:       global.Config.HttpCache.DB,
//		})),
//		middleware.WithHttpCacheGlobalSkipFields("v"),
//		middleware.WithHttpCacheRouteSkipFiledPolicy("/user/", true),
//	))
func HttpCache(opts ...httpcache.Option) gin.HandlerFunc {
	return httpcache.New(opts...)
}

// WithHttpCacheRedisStoreBy 通过地址设置缓存存储器 redis 连接
func WithHttpCacheRedisStoreBy(addr string, db uint) httpcache.Option {
	return httpcache.WithRedisStoreBy(addr, db)
}

// WithHttpCacheRedisStore 设置缓存存储器 redis 连接
func WithHttpCacheRedisStore(client *redis.Client) httpcache.Option {
	return httpcache.WithRedisStore(client)
}

// WithHttpCacheLogger 设置日志器
func WithHttpCacheLogger(log xlog.XLogger) httpcache.Option {
	return httpcache.WithLogger(log)
}

// WithHttpCacheDebug 是否启用 debug
func WithHttpCacheDebug(enabled bool) httpcache.Option {
	return httpcache.WithDebug(enabled)
}

// WithHttpCacheJWTOption jwt 鉴权参与缓存
func WithHttpCacheJWTOption(opt *jwt.Option) httpcache.Option {
	return httpcache.WithJWTOption(opt)
}

// WithHttpCacheGlobalDuration 全局缓存有效时间
func WithHttpCacheGlobalDuration(d time.Duration) httpcache.Option {
	return httpcache.WithGlobalCacheDuration(d)
}

// WithHttpCacheGlobalHeaderKeys 全局用于计算缓存的 Header
func WithHttpCacheGlobalHeaderKeys(keys []string) httpcache.Option {
	return httpcache.WithAppendGlobalHeaderKey(keys...)
}

// WithHttpCacheGlobalHeaderKey 全局用于计算缓存的 Header
func WithHttpCacheGlobalHeaderKey(key string) httpcache.Option {
	return httpcache.WithAppendGlobalHeaderKey(key)
}

// WithHttpCacheGlobalSkipFields 全局计算缓存的忽略字段
func WithHttpCacheGlobalSkipFields(field string, fields ...string) httpcache.Option {
	return httpcache.WithGlobalSkipQueryFields(append([]string{field}, fields...)...)
}

// WithHttpCacheKeyPrefix 自定义缓存前缀
func WithHttpCacheKeyPrefix(str string) httpcache.Option {
	return httpcache.WithCacheKeyPrefix(str)
}

// WithoutHttpCacheResponseHeader 是否不缓存响应 header。默认是（即：不缓存）
func WithoutHttpCacheResponseHeader(without bool) httpcache.Option {
	return httpcache.WithoutResponseHeader(without)
}

// WithHttpCacheRoutePolicy 路由策略
func WithHttpCacheRoutePolicy(route string, withToken bool, fields ...string) httpcache.Option {
	return httpcache.WithRoutePolicy(route, withToken, fields...)
}

// WithHttpCacheRouteRule 路由规则
func WithHttpCacheRouteRule(route string, withToken bool, duration time.Duration, fields ...string) httpcache.Option {
	return httpcache.WithRouteRule(route, withToken, duration, fields...)
}

// WithHttpCacheRouteSkipFiledPolicy 带忽略字段的路策略
func WithHttpCacheRouteSkipFiledPolicy(route string, withToken bool, skipFields ...string) httpcache.Option {
	return httpcache.WithRouteSkipFiledPolicy(route, withToken, skipFields...)
}

// WithHttpCacheRouteSkipFiledRule 带忽略字段的路由规则
func WithHttpCacheRouteSkipFiledRule(route string, withToken bool, duration time.Duration, fields ...string) httpcache.Option {
	return httpcache.WithRouteSkipFiledRule(route, withToken, duration, fields...)
}
