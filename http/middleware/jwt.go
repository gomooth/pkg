package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/gomooth/pkg/http/jwt"
	mjwt "github.com/gomooth/pkg/http/middleware/internal/jwt"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
)

// JWTWith jwt 鉴权中间件
// 在用户登录成功后，配合 jwt.NewToken 生成 token
func JWTWith(secret []byte, roleConvert httpcontext.ToRole, opts ...func(*jwt.Option)) gin.HandlerFunc {
	opt := jwt.NewOption(secret, roleConvert, opts...)
	return func(c *gin.Context) {
		if err := mjwt.NewHandler(c, opt).Handle(); err != nil {
			if !opt.SilentMode() {
				_ = c.AbortWithError(http.StatusUnauthorized, xerror.NewXCode(xcode.Unauthorized, "unauthorized"))
			} else {
				c.Abort()
			}
			return
		}
		c.Next()
	}
}

// JWTStatefulWith 有状态的 jwt 鉴权中间件
// 需要配合 jwt.NewStatefulToken 使用（在用户登录成功后，调用该函数创建token）
//
// usage:
//
//	 ra := router.Group(
//			"/user",
//			middleware.JWTStatefulWith(
//				[]byte(global.Config.App.Secret),
//				NewRole,
//				jwtstore.NewSingleRedisStore(global.SessionStoreClient),
//				jwt.WithRefreshDuration(5*time.Minute),
//			),
//			middleware.Roles([]types.IRole{global.RoleBroker, global.RoleStar, global.RoleMember}),
//		)
func JWTStatefulWith(secret []byte, roleConvert httpcontext.ToRole, store jwt.StatefulStore, opts ...func(*jwt.Option)) gin.HandlerFunc {
	opt := jwt.NewOption(secret, roleConvert, opts...)
	return func(c *gin.Context) {
		if err := mjwt.NewStatefulHandler(c, opt, store).Handle(); err != nil {
			if !opt.SilentMode() {
				_ = c.AbortWithError(http.StatusUnauthorized, xerror.NewXCode(xcode.Unauthorized, "unauthorized"))
			} else {
				c.Abort()
			}
			return
		}
		c.Next()
	}
}

// JWTStatefulWithout 有状态的 jwt 鉴权中间件，仅校验 jwt 是否合法，不校验状态
// 需要配合 jwt.NewStatefulToken 使用（在用户登录成功后，调用该函数创建token）
func JWTStatefulWithout(secret []byte, roleConvert httpcontext.ToRole, opts ...func(*jwt.Option)) gin.HandlerFunc {
	opt := jwt.NewOption(secret, roleConvert, opts...)
	return func(c *gin.Context) {
		if err := mjwt.NewStatefulHandler(c, opt, nil).Handle(); err != nil {
			if !opt.SilentMode() {
				_ = c.AbortWithError(http.StatusUnauthorized, xerror.NewXCode(xcode.Unauthorized, "unauthorized"))
			} else {
				c.Abort()
			}
			return
		}
		c.Next()
	}
}
