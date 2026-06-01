package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func TestRateLimit_Allowed(t *testing.T) {
	rate, err := limiter.NewRateFromFormatted("5-M")
	assert.NoError(t, err)

	lstore := memory.NewStore()
	instance := limiter.New(lstore, rate)

	r := gin.New()
	r.Use(RateLimit("test-key", instance))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_Rejected(t *testing.T) {
	rate, err := limiter.NewRateFromFormatted("2-M")
	assert.NoError(t, err)

	store := memory.NewStore()
	instance := limiter.New(store, rate)

	r := gin.New()
	r.Use(RateLimit("rate-reject-key", instance))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 消耗允许的 2 次请求
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 第 3 次请求应被限流
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "Too many requests")
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

func TestDynamicRateLimit(t *testing.T) {
	rate, err := limiter.NewRateFromFormatted("2-M")
	assert.NoError(t, err)

	store := memory.NewStore()
	instance := limiter.New(store, rate)

	r := gin.New()
	r.Use(DynamicRateLimit(func(c *gin.Context) string {
		return c.Query("tenant")
	}, instance))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// tenant=A 的两次请求都应通过
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test?tenant=A", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// tenant=A 第3次请求应被限流
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test?tenant=A", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// tenant=B 的请求不受影响
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/test?tenant=B", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestIPRateLimit(t *testing.T) {
	rate, err := limiter.NewRateFromFormatted("2-M")
	assert.NoError(t, err)

	store := memory.NewStore()
	instance := limiter.New(store, rate)

	r := gin.New()
	r.Use(IPRateLimit(instance))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// 同一 IP 发送两次请求，均应通过
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 第3次请求应被限流
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
}
