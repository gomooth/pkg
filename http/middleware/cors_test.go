package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCORS_DefaultOptions(t *testing.T) {
	r := gin.New()
	r.Use(CORS())
	r.OPTIONS("/test", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	r.ServeHTTP(w, req)

	// 默认未配置 AllowOriginFunc 时，应拒绝跨域（gin-contrib/cors 返回 403）
	assert.NotContains(t, w.Header().Get("Access-Control-Allow-Origin"), "http://example.com")
}

func TestCORS_WithAllowOriginFunc(t *testing.T) {
	r := gin.New()
	r.Use(CORS(WithCORSAllowOriginFunc(func(origin string) bool {
		return origin == "http://allowed.com"
	})))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 允许的源
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://allowed.com")
	r.ServeHTTP(w, req)
	assert.Equal(t, "http://allowed.com", w.Header().Get("Access-Control-Allow-Origin"))

	// 不允许的源
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("Origin", "http://blocked.com")
	r.ServeHTTP(w2, req2)
	assert.NotEqual(t, "http://blocked.com", w2.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WithAllowHeaders(t *testing.T) {
	r := gin.New()
	r.Use(CORS(
		WithCORSAllowOriginFunc(func(origin string) bool { return true }),
		WithCORSAllowHeaders("X-Custom-Key"),
	))
	r.OPTIONS("/test", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "X-Custom-Key")
	r.ServeHTTP(w, req)

	allowed := w.Header().Get("Access-Control-Allow-Headers")
	assert.Contains(t, allowed, "X-Custom-Key")
}

func TestCORS_WithMaxAge(t *testing.T) {
	r := gin.New()
	r.Use(CORS(
		WithCORSAllowOriginFunc(func(origin string) bool { return true }),
		WithCORSMaxAge(24*time.Hour),
	))
	r.OPTIONS("/test", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	r.ServeHTTP(w, req)

	maxAge := w.Header().Get("Access-Control-Max-Age")
	assert.NotEmpty(t, maxAge)
}
