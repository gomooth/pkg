package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/middleware/internal/cors"
)

// CORS 跨域处理
//
// usage:
//
//	  r.Use(middleware.CORS())
//
//		 r.Use(middleware.CORS(
//		 	middleware.WithCORSAllowOriginFunc(func(origin string) bool {
//		 		//return origin == "https://xxxx.com"
//		 		return true
//		 	}),
//		 	middleware.WithCORSAllowHeaders("X-Custom-Key"),
//		 	middleware.WithCORSExposeHeaders("X-Custom-Key"),
//		 	middleware.WithCORSMaxAge(24*time.Hour),
//		 ))
func CORS(opts ...cors.Option) gin.HandlerFunc {
	return cors.New(opts...)
}

// WithCORSAllowOriginFunc 设置允许的源
func WithCORSAllowOriginFunc(fun func(origin string) bool) cors.Option {
	return cors.WithAllowOriginFunc(fun)
}

// WithCORSAllowMethods 设置允许的 Method
// 默认允许方法有：GET, POST, PUT, DELETE, OPTIONS
func WithCORSAllowMethods(methods ...string) cors.Option {
	return cors.WithAllowMethods(methods...)
}

// WithCORSHeaders 设置允许的请求头
// 该操作会同时进行 WithCORSAllowHeaders, WithCORSExposeHeaders 设置
func WithCORSHeaders(keys ...string) cors.Option {
	return cors.WithHeaders(keys...)
}

// WithCORSAllowHeaders 设置服务器允许客户端在跨域请求中携带的请求头
// 如果客户端发送的请求头不在允许列表中，浏览器会拒绝该请求（触发 CORS 错误）。
// 默认允许的请求头有：
//
//	Origin, Content-Type, Accept, User-Agent, Cookie, Authorization,
//	X-Requested-With, X-Auth-Token, X-Token
func WithCORSAllowHeaders(keys ...string) cors.Option {
	return cors.WithAllowHeaders(keys...)
}

// WithCORSExposeHeaders 指定客户端 JavaScript 代码可以访问的响应头
// 如果需要访问自定义头，必须通过该方法声明。否则无法获取对应值
// 默认允许访问的响应头有：
//
//	Authorization, Content-MD5
//	Link, X-Pagination-Info, X-PaginateTotal-Count, X-More-Resource
//	X-Error-Code, X-Error-Data
//	X-Token
func WithCORSExposeHeaders(keys ...string) cors.Option {
	return cors.WithExposeHeaders(keys...)
}

// WithCORSMaxAge 指定预检请求（Preflight Request, OPTIONS）的缓存时间。默认为 12小时。
// 在缓存有效期内，浏览器不会对同一跨域请求重复发送预检请求，直接使用缓存结果。
// 设置合适的参数，可以优化高频跨域请求的性能（如 API 频繁调用）。
// 一般，24小时内，同一跨域请求（相同 URL 和方法）不需要再次发送 OPTIONS 预检请求。
func WithCORSMaxAge(d time.Duration) cors.Option {
	return cors.WithMaxAge(d)
}
