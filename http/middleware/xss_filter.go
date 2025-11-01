package middleware

import (
	"github.com/gin-gonic/gin"
	mxss "github.com/gomooth/pkg/http/middleware/internal/xss"
	"github.com/gomooth/pkg/http/xss"
)

// XSSFilter XSS 过滤
//
// usage:
//
//	r.Use(middleware.XSSFilter(
//		//middleware.XSSDebug(),
//		middleware.WithXSSGlobalPolicy(xss.PolicyStrict),
//		middleware.WithXSSGlobalFieldPolicy(xss.PolicyUGC, "content", "details"),
//		middleware.WithXSSGlobalSkipFields("password"),
//		middleware.WithXSSRoutePolicy("admin", xss.PolicyUGC),
//		middleware.WithXSSRoutePolicy("/callback/", xss.PolicyNone),
//		middleware.WithXSSRoutePolicy("/endpoint", xss.PolicyNone),
//		middleware.WithXSSRoutePolicy("/ping", xss.PolicyNone),
//		middleware.WithXSSRouteFieldPolicy("/user/", xss.PolicyUGC, "content"),
//	))
func XSSFilter(opts ...mxss.Option) gin.HandlerFunc {
	return mxss.New(opts...)
}

// WithXSSGlobalPolicy 指定全局过滤策略
func WithXSSGlobalPolicy(p xss.Policy) mxss.Option {
	return mxss.WithGlobalPolicy(p)
}

// WithXSSGlobalFieldPolicy 指定全局字段过滤策略
func WithXSSGlobalFieldPolicy(p xss.Policy, fields ...string) mxss.Option {
	return mxss.WithGlobalFieldPolicy(p, fields...)
}

// WithXSSDebug 设置调试模式
func WithXSSDebug(enabled bool) mxss.Option {
	return mxss.WithDebug(enabled)
}

// WithTrimSpaceEnabled 设置是否开启过滤前后空格
func WithTrimSpaceEnabled(enabled bool) mxss.Option {
	return mxss.WithTrimSpaceEnabled(enabled)
}

// WithXSSGlobalSkipFields 指定全局忽略字段
// 默认会自动跳过密码相关的字段，包括：
//
//	password, newPassword, oldPassword, confirmedPassword,
//	pwd, newPwd, oldPwd, confirmedPwd
func WithXSSGlobalSkipFields(fields ...string) mxss.Option {
	return mxss.WithGlobalSkipFields(fields...)
}

// WithXSSRoutePolicy 指定路由策略
// routeRule 路由规则，如果路由包含该字符串则匹配成功
func WithXSSRoutePolicy(routeRule string, policy xss.Policy, skipFields ...string) mxss.Option {
	return mxss.WithRoutePolicy(routeRule, policy, skipFields...)
}

// WithXSSRouteFieldPolicy 指定路由的字段策略
// routeRule 路由规则，如果路由包含该字符串则匹配成功
func WithXSSRouteFieldPolicy(routeRule string, policy xss.Policy, fields ...string) mxss.Option {
	return mxss.WithRouteFieldPolicy(routeRule, policy, fields...)
}
