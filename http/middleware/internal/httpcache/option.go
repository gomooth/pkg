package httpcache

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/middleware/internal/httpcache/store"

	"github.com/redis/go-redis/v9"

	"github.com/gomooth/utils/sliceutil"
)

type Option func(*handler)

func WithRedisStoreBy(addr string, db uint) Option {
	return func(c *handler) {
		if len(addr) != 0 {
			c.store = store.NewOwnedRedisStore(redis.NewClient(&redis.Options{
				Network: "tcp",
				Addr:    addr,
				DB:      int(db),
			}))
		}
	}
}

func WithRedisStore(client *redis.Client) Option {
	return func(c *handler) {
		if client != nil {
			c.store = store.NewRedisStore(client)
		}
	}
}

func WithLogger(log *slog.Logger) Option {
	return func(c *handler) {
		c.log = log
	}
}

func WithDebug(enabled bool) Option {
	return func(c *handler) {
		c.debug = enabled
	}
}

// WithUserIDFunc 设置从请求上下文提取用户 ID 的回调函数，用于 withToken 路由构建用户级缓存 key。
func WithUserIDFunc(fn func(*gin.Context) (uint, error)) Option {
	return func(c *handler) {
		c.userIDFunc = fn
	}
}

func WithGlobalCacheDuration(d time.Duration) Option {
	return func(c *handler) {
		c.globalCacheDuration = d
	}
}

func WithGlobalHeaderKey(keys []string) Option {
	return func(c *handler) {
		c.globalHeaderKeys = keys
	}
}

func WithAppendGlobalHeaderKey(keys ...string) Option {
	return func(c *handler) {
		globals := c.globalHeaderKeys
		if len(globals) == 0 {
			globals = []string{}
		}
		c.globalHeaderKeys = append(globals, keys...)
	}
}

func WithGlobalSkipQueryFields(fields ...string) Option {
	return func(c *handler) {
		for _, field := range fields {
			c.globalSkipFields[field] = struct{}{}
		}
	}
}

func WithCacheKeyPrefix(str string) Option {
	return func(c *handler) {
		c.prefixKey = str
	}
}

func WithoutResponseHeader(without bool) Option {
	return func(c *handler) {
		c.withoutResponseHeader = without
	}
}

// WithSingleFlightTimeout 设置 singleflight 的超时时间。
// 默认 10ms（约 100QPS）。设为 0 禁用 singleflight 自动 Forget。
func WithSingleFlightTimeout(d time.Duration) Option {
	return func(c *handler) {
		c.singleFlightTimeout = d
	}
}

// WithRoutePolicy 路由策略
func WithRoutePolicy(route string, withToken bool, fields ...string) Option {
	return withRouteRule(route, withToken, 0, fields, nil, nil)
}

// WithRouteRule 路由规则
func WithRouteRule(route string, withToken bool, duration time.Duration, fields ...string) Option {
	return withRouteRule(route, withToken, duration, fields, nil, nil)
}

// WithRouteSkipFiledPolicy 带忽略字段的路由策略
func WithRouteSkipFiledPolicy(route string, withToken bool, skipFields ...string) Option {
	return withRouteRule(route, withToken, 0, nil, nil, skipFields)
}

// WithRouteSkipFiledRule 带忽略字段的路由规则
func WithRouteSkipFiledRule(route string, withToken bool, duration time.Duration, skipFields ...string) Option {
	return withRouteRule(route, withToken, duration, nil, nil, skipFields)
}

func withRouteRule(route string, withToken bool, duration time.Duration, fields, headerKeys, skipFields []string) Option {
	return func(c *handler) {
		// 先记录顺序
		c.routeList = append(c.routeList, route)
		c.routeList = sliceutil.Unique(c.routeList)

		if c.routePolicies == nil {
			c.routePolicies = make(map[string]*ruleItem, 0)
		}

		rule, ok := c.routePolicies[route]
		if !ok {
			rule = &ruleItem{}
		}

		rule.withToken = withToken
		rule.duration = duration

		if fields != nil {
			if rule.fields == nil {
				rule.fields = make(map[string]struct{}, 0)
			}
			for _, field := range fields {
				rule.fields[field] = struct{}{}
			}
		}

		if headerKeys != nil {
			if rule.headerKeys == nil {
				rule.headerKeys = make([]string, 0)
			}
			rule.headerKeys = append(rule.headerKeys, headerKeys...)
		}

		if skipFields != nil {
			if rule.skipFields == nil {
				rule.skipFields = make(map[string]struct{}, 0)
			}
			for _, field := range skipFields {
				rule.skipFields[field] = struct{}{}
			}
		}

		// 写入规则
		c.routePolicies[route] = rule
	}
}
