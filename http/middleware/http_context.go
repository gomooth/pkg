package middleware

import (
	"bytes"
	"io"

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
		stx := httpcontext.NewContext(
			httpcontext.WithRawRequestBody(body),
		)

		// 重新写入 request body
		ctx.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		// 注册自定义上下文
		ctx.Set(httpcontext.ContextKey, stx)

		ctx.Next()
	}
}
