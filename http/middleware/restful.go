package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/middleware/internal/restful"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
)

// RESTFul Restful 标准检测解析中间件
func RESTFul(version string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if err := restful.New(ctx, version).Handle(); err != nil {
			slog.Warn("not support accept", slog.String("component", "restful"), slog.String("error", err.Error()))
			_ = ctx.AbortWithError(http.StatusBadRequest, xerror.NewXCode(xcode.RequestParamError, "not support accept"))
			return
		}

		ctx.Next()
	}
}

type IgnorePath struct {
	Path   string
	Method string
}

// RESTFulWithIgnores 忽略指定 path 的Restful 标准检测解析中间件
// 一般，用在部分直接下载或浏览器直接访问的接口
func RESTFulWithIgnores(version string, ignorePaths ...IgnorePath) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		for _, ignore := range ignorePaths {
			if ignore.Path == ctx.FullPath() &&
				strings.ToLower(ctx.Request.Method) == strings.ToLower(ignore.Method) {
				ctx.Next()
				return
			}
		}

		if err := restful.New(ctx, version).Handle(); err != nil {
			slog.Warn("not support accept", slog.String("component", "restful"), slog.String("error", err.Error()))
			_ = ctx.AbortWithError(http.StatusBadRequest, xerror.NewXCode(xcode.RequestParamError, "not support accept"))
			return
		}

		ctx.Next()
	}
}
