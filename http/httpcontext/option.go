package httpcontext

import (
	"context"
	"fmt"
)

func WithParent(parent context.Context) func(*httpContext) {
	return func(c *httpContext) {
		if parent != nil {
			c.parent = parent
		}
	}
}

func WithUser(user *User) func(*httpContext) {
	return func(c *httpContext) {
		c.user = user
	}
}

func WithTrace(traceID, spanID string) func(*httpContext) {
	return func(c *httpContext) {
		c.traceID = fmt.Sprintf("00-%s-%s-01", traceID, spanID)
	}
}

func WithRawRequestBody(body []byte) func(*httpContext) {
	return func(c *httpContext) {
		WithData(RequestRawBodyDataKey, body)(c)
	}
}

func WithData(key string, value any) func(*httpContext) {
	return func(c *httpContext) {
		c.parent = context.WithValue(c.parent, ctxKey(key), value)
	}
}
