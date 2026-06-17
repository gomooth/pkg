package httpcontext

import (
	"context"
)

// WithParent 设置上下文的父 context.Context，默认为 context.Background()
func WithParent(parent context.Context) ContextOption {
	return func(o *httpContextOption) {
		if parent != nil {
			o.parent = parent
		}
	}
}

// WithUser 设置上下文中的用户信息
func WithUser(user *User) ContextOption {
	return func(o *httpContextOption) {
		o.user = user
	}
}

// WithRawRequestBody 设置原始请求体数据，存入上下文以供后续读取
func WithRawRequestBody(body []byte) ContextOption {
	return func(o *httpContextOption) {
		WithData(RequestRawBodyDataKey, body)(o)
	}
}

// WithData 向上下文存入键值对数据，基于 context.WithValue 存储
func WithData(key string, value any) ContextOption {
	return func(o *httpContextOption) {
		o.data = append(o.data, contextKV{key: key, value: value})
	}
}