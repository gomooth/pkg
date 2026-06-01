package httpcontext

import (
	"context"
)

// WithParent 设置上下文的父 context.Context，默认为 context.Background()
func WithParent(parent context.Context) func(*httpContext) {
	return func(c *httpContext) {
		if parent != nil {
			c.parent = parent
		}
	}
}

// WithUser 设置上下文中的用户信息
func WithUser(user *User) func(*httpContext) {
	return func(c *httpContext) {
		c.user = user
	}
}

// WithRawRequestBody 设置原始请求体数据，存入上下文以供后续读取
func WithRawRequestBody(body []byte) func(*httpContext) {
	return func(c *httpContext) {
		WithData(RequestRawBodyDataKey, body)(c)
	}
}

// WithData 向上下文存入键值对数据，基于 context.WithValue 存储
func WithData(key string, value any) func(*httpContext) {
	return func(c *httpContext) {
		c.parent = context.WithValue(c.parent, ctxKey(key), value)
	}
}
