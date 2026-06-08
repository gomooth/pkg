package httpcontext

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

// ctxKey 是 httpContext 内部使用的 context key 类型，
// 避免与标准库或其他包的 string key 冲突。
type ctxKey string

// IHttpContext 自定义 HTTP 请求上下文接口，封装用户信息、角色判断和上下文数据存取
type IHttpContext interface {
	context.Context

	// User 用户信息（返回防御性拷贝，修改返回值不影响内部状态）
	User() *User
	// IsRole 判断用户角色
	IsRole(role IRole) bool
	// SetUser 设置用户信息（仅在请求处理链中顺序调用，不可并发调用）
	SetUser(user User) IHttpContext
	// Set 设置参数，通过 context.WithValue 存入 parent 链（仅在请求处理链中顺序调用，不可并发调用）
	Set(key string, value any) IHttpContext
	// StorageTo 将已变更的数据，存储到 gin 上下文中，继续传输
	StorageTo(ctx *gin.Context) bool
}

type httpContext struct {
	parent context.Context
	user   *User
}

var _ IHttpContext = (*httpContext)(nil)

// NewContext 创建自定义 HTTP 上下文，通过选项函数进行初始化配置
func NewContext(opts ...func(*httpContext)) IHttpContext {
	c := &httpContext{
		parent: context.Background(),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *httpContext) User() *User {
	if c.user == nil {
		return nil
	}
	clone := c.user.Clone()
	return &clone
}

// IsRole 判断用户角色
func (c *httpContext) IsRole(role IRole) bool {
	if c.user == nil {
		return false
	}
	return c.user.Is(role)
}

func (c *httpContext) Deadline() (deadline time.Time, ok bool) {
	return c.parent.Deadline()
}

func (c *httpContext) Done() <-chan struct{} {
	return c.parent.Done()
}

func (c *httpContext) Err() error {
	return c.parent.Err()
}

func (c *httpContext) Value(key any) any {
	// 支持 ctxKey 类型查找
	if k, ok := key.(ctxKey); ok {
		return c.parent.Value(k)
	}
	// 兼容外部用 string key 查找
	if k, ok := key.(string); ok {
		return c.parent.Value(ctxKey(k))
	}
	return c.parent.Value(key)
}

// Set 设置参数，通过 context.WithValue 存入 parent 链
func (c *httpContext) Set(key string, value any) IHttpContext {
	if len(key) == 0 {
		return c
	}
	c.parent = context.WithValue(c.parent, ctxKey(key), value)
	return c
}

// SetUser 设置用户信息，内部会进行深拷贝
func (c *httpContext) SetUser(user User) IHttpContext {
	clone := user.Clone()
	c.user = &clone
	return c
}

// StorageTo 将已变更的数据，存储到 gin 上下文中，继续传输
func (c *httpContext) StorageTo(ctx *gin.Context) bool {
	ctx.Set(ContextKey, c)
	return true
}
