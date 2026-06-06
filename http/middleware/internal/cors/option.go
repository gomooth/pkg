package cors

import "time"

type Option func(*handler)

func WithAllowOriginFunc(fun func(origin string) bool) Option {
	return func(ch *handler) {
		ch.allowOriginFunc = fun
	}
}

func WithAllowMethods(methods ...string) Option {
	return func(ch *handler) {
		if len(methods) > 0 {
			ch.allowMethods = append(ch.allowMethods, methods...)
		}
	}
}

func WithHeaders(keys ...string) Option {
	return func(ch *handler) {
		if len(keys) > 0 {
			ch.allowHeaders = append(ch.allowHeaders, keys...)
			ch.exposeHeaders = append(ch.exposeHeaders, keys...)
		}
	}
}

func WithAllowHeaders(keys ...string) Option {
	return func(ch *handler) {
		if len(keys) > 0 {
			ch.allowHeaders = append(ch.allowHeaders, keys...)
		}
	}
}

func WithExposeHeaders(keys ...string) Option {
	return func(ch *handler) {
		if len(keys) > 0 {
			ch.exposeHeaders = append(ch.exposeHeaders, keys...)
		}
	}
}

func WithMaxAge(d time.Duration) Option {
	return func(ch *handler) {
		if d > 0 {
			ch.maxAge = d
		}
	}
}

// WithCORSAllowCredentials 设置是否允许携带凭证（Cookie、Authorization header）。
// 默认 true。设为 false 时浏览器不会在跨域请求中发送凭证。
func WithCORSAllowCredentials(enabled bool) Option {
	return func(ch *handler) {
		ch.allowCredentials = &enabled
	}
}
