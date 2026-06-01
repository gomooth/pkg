package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/framework/telemetry"
	"github.com/gomooth/pkg/http/httpcontext"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// HttpContext 注入自定义上下文，同时自动创建 OTel Span 作为链路追踪入口。
//
// usage:
//
//	r.Use(middleware.HttpContext())
func HttpContext() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// 取出 request body
		body, _ := ctx.GetRawData()

		// 创建 OTel Span
		tracer := telemetry.Tracer("http")
		spanName := fmt.Sprintf("%s %s", ctx.Request.Method, ctx.FullPath())
		if spanName == "" {
			spanName = fmt.Sprintf("%s %s", ctx.Request.Method, "unknown")
		}

		spanCtx, span := tracer.Start(
			ctx.Request.Context(), spanName,
			trace.WithAttributes(
				attribute.String("http.method", ctx.Request.Method),
				attribute.String("http.url", ctx.Request.URL.String()),
				attribute.String("http.scheme", ctx.Request.URL.Scheme),
				attribute.String("http.host", ctx.Request.Host),
			),
			trace.WithSpanKind(trace.SpanKindServer),
		)
		defer span.End()

		// defer 记录响应状态码
		defer func() {
			span.SetAttributes(attribute.Int("http.status_code", ctx.Writer.Status()))
			if ctx.Writer.Status() >= 400 {
				span.SetStatus(codes.Error, http.StatusText(ctx.Writer.Status()))
			} else {
				span.SetStatus(codes.Ok, "")
			}
		}()

		// 替换 request context 为包含 span 的 context
		ctx.Request = ctx.Request.WithContext(spanCtx)

		// 创建 httpContext
		stx := httpcontext.NewContext(
			httpcontext.WithParent(spanCtx),
			httpcontext.WithRawRequestBody(body),
		)

		// 重新写入 request body
		ctx.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		// 注册自定义上下文
		ctx.Set(httpcontext.ContextKey, stx)

		ctx.Next()
	}
}
