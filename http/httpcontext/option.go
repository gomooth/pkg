package httpcontext

import "fmt"

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

func WithData(key string, value interface{}) func(*httpContext) {
	return func(c *httpContext) {
		if c.storage == nil {
			c.storage = make(map[string]interface{})
		}
		c.storage[key] = value
	}
}
