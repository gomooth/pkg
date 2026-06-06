package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"

	"github.com/gin-gonic/gin"
)

// WithRole 角色权限中间件
func WithRole(role httpcontext.IRole, roles ...httpcontext.IRole) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		svrCtx, err := httpcontext.Parse(ctx)
		if err != nil {
			slog.Error("role check failed: context convert failed", slog.String("component", "role"), slog.String("error", err.Error()))
			_ = ctx.AbortWithError(http.StatusForbidden, xerror.NewXCode(xcode.Forbidden, "context convert failed"))
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
			slog.Warn("role check failed: insufficient permissions", slog.String("component", "role"))
			_ = ctx.AbortWithError(http.StatusForbidden, xerror.NewXCode(xcode.Forbidden, "role error"))
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
		if htx, err := httpcontext.Parse(ctx); err == nil {
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
		htx, err := httpcontext.Parse(ctx)
		if err != nil {
			slog.Error("role check failed: context convert failed", slog.String("component", "role"), slog.String("error", err.Error()))
			ctx.AbortWithStatus(http.StatusForbidden)
			return
		}

		isRole := false
		rs := append([]httpcontext.IRole{role}, roles...)
		for _, r := range rs {
			if htx.IsRole(r) {
				isRole = true
				break
			}
		}

		if isRole {
			handler(ctx)
			ctx.Next()
			return
		}

		slog.Warn("role check failed: insufficient permissions, aborting", slog.String("component", "role"))
		ctx.AbortWithStatus(http.StatusForbidden)
	}
}
