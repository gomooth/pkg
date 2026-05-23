package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/gomooth/pkg/http/httpcontext"

	"github.com/gin-gonic/gin"
)

// HttpContext 注入自定义上下文
//
// usage:
//
//	r.Use(middleware.HttpContext())
func HttpContext() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 取出 request body
		body, _ := ctx.GetRawData()
		stx, err := httpcontext.NewContext(
			httpcontext.WithParent(ctx.Request.Context()),
			httpcontext.WithRawRequestBody(body),
		)
		if err != nil {
			ctx.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		// 重新写入 request body
		ctx.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		// 注册自定义上下文
		ctx.Set(httpcontext.ContextKey, stx)

		ctx.Next()
	}
}
