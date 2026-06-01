package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestWithCORSAllowMethods(t *testing.T) {
	r := gin.New()
	r.Use(CORS(
		WithCORSAllowOriginFunc(func(origin string) bool { return true }),
		WithCORSAllowMethods("PATCH"),
	))
	r.OPTIONS("/test", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "PATCH")
	r.ServeHTTP(w, req)

	allowed := w.Header().Get("Access-Control-Allow-Methods")
	assert.Contains(t, allowed, "PATCH")
}

func TestWithCORSExposeHeaders(t *testing.T) {
	r := gin.New()
	r.Use(CORS(
		WithCORSAllowOriginFunc(func(origin string) bool { return true }),
		WithCORSExposeHeaders("X-Custom-Response-Header"),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	r.ServeHTTP(w, req)

	exposed := w.Header().Get("Access-Control-Expose-Headers")
	assert.Contains(t, exposed, "X-Custom-Response-Header")
}

func TestWithCORSHeaders(t *testing.T) {
	r := gin.New()
	r.Use(CORS(
		WithCORSAllowOriginFunc(func(origin string) bool { return true }),
		WithCORSHeaders("X-Shared-Key"),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	r.ServeHTTP(w, req)

	// WithCORSHeaders 同时设置 AllowHeaders 和 ExposeHeaders
	exposed := w.Header().Get("Access-Control-Expose-Headers")
	assert.Contains(t, exposed, "X-Shared-Key")
}
