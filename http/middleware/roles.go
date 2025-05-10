package middleware

import (
	"fmt"
	"net/http"

	"github.com/gomooth/pkg/http/httpcontext"

	"github.com/gin-gonic/gin"
)

// WithRole 角色权限中间件
func WithRole(role httpcontext.IRole, roles ...httpcontext.IRole) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		svrCtx, err := httpcontext.MustParse(ctx)
		if nil != err {
			fmt.Println("role error: context convert failed")
			_ = ctx.AbortWithError(http.StatusForbidden, fmt.Errorf("context convert failed"))
			return
		}

		isRole := false
		rs := append([]httpcontext.IRole{role}, roles...)
		for _, r := range rs {
			if svrCtx.IsRole(r) {
				isRole = true
				break
			}
		}
		if !isRole {
			fmt.Println("role error")
			_ = ctx.AbortWithError(http.StatusForbidden, fmt.Errorf("role error"))
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}

// RoleFunc 角色控制器中间件。
// 如果用户满足指定角色要求，则调用 handler，并在完成后进入下一个中间件；
// 如果用户不满足指定角色要求，则直接进入下一个中间件
// 一般，在同一路由针对不同角色处理逻辑完成不同的场景很实用。
func RoleFunc(handler gin.HandlerFunc, role httpcontext.IRole, roles ...httpcontext.IRole) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if htx, err := httpcontext.MustParse(ctx); nil == err {
			isRole := false
			roles = append([]httpcontext.IRole{role}, roles...)
			for _, r := range roles {
				if htx.IsRole(r) {
					isRole = true
					break
				}
			}
			if isRole {
				handler(ctx)
			}
		}

		ctx.Next()
	}
}

// RoleFuncAbort 角色控制器独占中间件。
// 如果用户符合指定角色，则使用调用 handler，并在完成后进入下一个中间件；
// 如果用户不满足指定角色要求，则中断链路，返回 http status 403 错误
func RoleFuncAbort(handler gin.HandlerFunc, role httpcontext.IRole, roles ...httpcontext.IRole) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if htx, err := httpcontext.MustParse(ctx); nil == err {
			isRole := false
			roles = append([]httpcontext.IRole{role}, roles...)
			for _, r := range roles {
				if htx.IsRole(r) {
					isRole = true
					break
				}
			}
			if isRole {
				handler(ctx)
				ctx.Next()
			}
		}
		fmt.Println("role error, abort")
		ctx.AbortWithStatus(http.StatusForbidden)
	}
}
