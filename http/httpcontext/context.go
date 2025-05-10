package httpcontext

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
)

type IHttpContext interface {
	context.Context

	// TraceID 链路追踪标志
	TraceID() string
	// User 用户信息
	User() *User
	// IsRole 判断用户角色
	IsRole(role IRole) bool
	// Set 设置参数
	// 会根据 value 的类型，自动设置对应属性的值，目前支持： User
	Set(key string, value interface{}) IHttpContext
	// StorageTo 将已变更的数据，存储到 gin 上下文中，继续传输
	StorageTo(ctx *gin.Context) bool
}

type httpContext struct {
	traceID string
	//version      ApiVersion   // 版本号
	//bodyProperty BodyProperty // 响应正文属性
	user *User // 用户信息

	storage map[string]interface{} // 存储变量
}

func NewContext(opts ...func(*httpContext)) IHttpContext {
	c := &httpContext{
		// 格式：版本-跟踪ID-父SpanID-标志
		traceID: fmt.Sprintf("00-%s-%s-01", makeTraceID(), makeSpanID()),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *httpContext) TraceID() string {
	return c.traceID
}

func (c *httpContext) User() *User {
	return c.user
}

// IsRole 判断用户角色
func (c *httpContext) IsRole(role IRole) bool {
	if c.user == nil {
		return false
	}

	for i := range c.user.Roles {
		if c.user.Roles[i] == role {
			return true
		}
	}

	return false
}

func (c *httpContext) Deadline() (deadline time.Time, ok bool) {
	return
}

func (c *httpContext) Done() <-chan struct{} {
	return nil
}

func (c *httpContext) Err() error {
	return nil
}

func (c *httpContext) Value(key interface{}) interface{} {
	if keyAsString, ok := key.(string); ok {
		val, _ := c.storage[keyAsString]
		return val
	}

	return nil
}

// Set 设置参数
// 会根据 value 的类型，自动设置对应属性的值，目前支持： ApiVersion, BodyProperty
func (c *httpContext) Set(key string, value interface{}) IHttpContext {
	if len(key) == 0 {
		return c
	}

	//// API 版本号
	//if av, ok := value.(ApiVersion); ok && av.Verify() {
	//	c.version = av
	//	return
	//}
	//
	//// 响应正文属性
	//if bp, ok := value.(BodyProperty); ok && bp.Verify() {
	//	c.bodyProperty = bp
	//	return
	//}

	// 用户信息
	if v, ok := value.(User); ok {
		c.user = &v
		return c
	}

	// 延迟初始化
	if c.storage == nil {
		c.storage = make(map[string]interface{})
	}

	c.storage[key] = value
	return c
}

// StorageTo 将已变更的数据，存储到 gin 上下文中，继续传输
func (c *httpContext) StorageTo(ctx *gin.Context) bool {
	ctx.Set(ContextKey, c)
	return true
}
