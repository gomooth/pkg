package xss

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/xss"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================
// 辅助：构建真实 JSON 请求体
// ============================================================

// benchJSONBody 返回一个包含 HTML 内容的典型 JSON 请求体
func benchJSONBody() string {
	return `{"username":"alice","bio":"<script>alert('xss')</script> Hello <b>world</b>","email":"alice@example.com","profile":{"nickname":"<img src=x onerror=alert(1)>Alice","signature":"<iframe src='evil.html'></iframe>Safe text"},"tags":["<script>evil()</script>","normal tag","<a href='javascript:bad()'>link</a>"]}`
}

// newBenchContext 创建带 JSON body 的 gin.Context，绑定到指定路由
func newBenchContext(method, path string, body io.Reader, contentType string) *gin.Context {
	w := httptest.NewRecorder()
	r := gin.New()

	var captured *gin.Context
	r.Handle(method, path, func(ctx *gin.Context) {
		captured = ctx
		ctx.Next()
	})

	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	r.ServeHTTP(w, req)
	return captured
}

// ============================================================
// BenchmarkHandler_Sanitize — 测量 XSS 过滤中间件对 JSON 请求体的处理开销
// ============================================================

func BenchmarkHandler_Sanitize_StrictPolicy(b *testing.B) {
	b.ReportAllocs()

	handler := New(WithGlobalPolicy(xss.PolicyStrict))
	body := benchJSONBody()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := newBenchContext(http.MethodPost, "/api/user", strings.NewReader(body), "application/json")
		handler(ctx)
	}
}

func BenchmarkHandler_Sanitize_UGCPolicy(b *testing.B) {
	b.ReportAllocs()

	handler := New(WithGlobalPolicy(xss.PolicyUGC))
	body := benchJSONBody()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := newBenchContext(http.MethodPost, "/api/user", strings.NewReader(body), "application/json")
		handler(ctx)
	}
}

// ============================================================
// BenchmarkHandler_Sanitize_QueryString — GET 请求查询参数过滤
// ============================================================

func BenchmarkHandler_Sanitize_QueryString_StrictPolicy(b *testing.B) {
	b.ReportAllocs()

	handler := New(WithGlobalPolicy(xss.PolicyStrict))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r := gin.New()
		var captured *gin.Context
		r.Handle(http.MethodGet, "/api/search", func(ctx *gin.Context) {
			captured = ctx
			ctx.Next()
		})
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=<script>alert(1)</script>&page=1&sort=<b>name</b>", nil)
		r.ServeHTTP(w, req)
		handler(captured)
	}
}

// ============================================================
// BenchmarkHandler_Sanitize_FormData — POST 表单数据过滤
// ============================================================

func BenchmarkHandler_Sanitize_FormData_StrictPolicy(b *testing.B) {
	b.ReportAllocs()

	handler := New(WithGlobalPolicy(xss.PolicyStrict))
	formBody := "username=<script>alert(1)</script>&bio=Hello+%3Cb%3Eworld%3C%2Fb%3E&email=test%40example.com"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := newBenchContext(http.MethodPost, "/api/user", strings.NewReader(formBody), "application/x-www-form-urlencoded")
		handler(ctx)
	}
}

// ============================================================
// BenchmarkHandler_Sanitize_MultipartFormData — multipart 表单过滤
// ============================================================

func BenchmarkHandler_Sanitize_Multipart_StrictPolicy(b *testing.B) {
	b.ReportAllocs()

	handler := New(WithGlobalPolicy(xss.PolicyStrict))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		_ = writer.WriteField("username", "alice")
		_ = writer.WriteField("bio", "<script>alert('xss')</script> Hello <b>world</b>")
		_ = writer.WriteField("description", "<iframe src='evil.html'></iframe>Safe content")
		writer.Close()

		ctx := newBenchContext(http.MethodPost, "/api/upload", &buf, writer.FormDataContentType())
		handler(ctx)
	}
}
