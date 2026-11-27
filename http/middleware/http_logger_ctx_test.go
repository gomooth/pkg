package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// ctxCaptureHandler 包装 slog.NewTextHandler，Handle 时捕获 ctx（同 dbutil 测试形态）
type ctxCaptureHandler struct {
	next slog.Handler
	got  *context.Context
}

func (h *ctxCaptureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *ctxCaptureHandler) Handle(ctx context.Context, r slog.Record) error {
	*h.got = ctx
	return h.next.Handle(ctx, r)
}

func (h *ctxCaptureHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }

func (h *ctxCaptureHandler) WithGroup(_ string) slog.Handler { return h }

// captureLogger 构造内存 slog.Logger，Handle 里记录 ctx
func captureLogger(got *context.Context) *slog.Logger {
	return slog.New(&ctxCaptureHandler{
		next: slog.NewTextHandler(&bytes.Buffer{}, nil),
		got:  got,
	})
}

// TestHttpLoggerLogsRequestContextSpan 用真实 sdktrace provider 起 span 替换 request ctx，
// 断言 HttpLogger 落日志时 ctx 内 span 仍可取（recording）
func TestHttpLoggerLogsRequestContextSpan(t *testing.T) {
	var gotCtx context.Context
	l := captureLogger(&gotCtx)
	tp := sdktrace.NewTracerProvider() // 本地 provider，span recording
	spanCtx, span := tp.Tracer("t").Start(context.Background(), "req")
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Request = c.Request.WithContext(spanCtx); c.Next() })
	r.Use(HttpLogger(HttpLoggerOption{Logger: l}))
	r.GET("/x", func(c *gin.Context) { c.Status(200) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	span.End()
	assert.True(t, trace.SpanFromContext(gotCtx).SpanContext().SpanID().IsValid(), "日志 ctx 应含有效 span")
}
