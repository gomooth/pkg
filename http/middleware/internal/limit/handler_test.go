package limit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/memory"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestRateLimiter_Allowed tests that requests within the rate limit are allowed
func TestRateLimiter_Allowed(t *testing.T) {
	rate, err := limiter.NewRateFromFormatted("5-M")
	assert.Nil(t, err)

	store := memory.NewStore()
	instance := limiter.New(store, rate)

	keyFn := func(c *gin.Context) string {
		return "test-key"
	}

	handler := RateLimiter(keyFn, instance)

	// First request should be allowed
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	handler(c)

	assert.False(t, c.IsAborted())
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestRateLimiter_Rejected tests that requests exceeding the rate limit are rejected
func TestRateLimiter_Rejected(t *testing.T) {
	// Rate limit: 2 requests per minute
	rate, err := limiter.NewRateFromFormatted("2-M")
	assert.Nil(t, err)

	store := memory.NewStore()
	instance := limiter.New(store, rate)

	keyFn := func(c *gin.Context) string {
		return "test-key-rejected"
	}

	handler := RateLimiter(keyFn, instance)

	// Consume the 2 allowed requests
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		handler(c)
		assert.False(t, c.IsAborted(), "request %d should be allowed", i+1)
	}

	// Third request should be rejected (429 Too Many Requests)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	handler(c)

	assert.True(t, c.IsAborted())
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Contains(t, w.Body.String(), "Too many requests")

	// Check rate limit headers are set
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
	assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
}

// TestRateLimiter_DifferentKeys tests that different keys have independent rate limits
func TestRateLimiter_DifferentKeys(t *testing.T) {
	rate, err := limiter.NewRateFromFormatted("1-M")
	assert.Nil(t, err)

	store := memory.NewStore()
	instance := limiter.New(store, rate)

	counter := 0
	keyFn := func(c *gin.Context) string {
		counter++
		if counter <= 1 {
			return "key-a"
		}
		return "key-b"
	}

	handler := RateLimiter(keyFn, instance)

	// First request with key-a
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	handler(c1)
	assert.False(t, c1.IsAborted())

	// Second request with key-b should also be allowed (different key, separate limit)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	handler(c2)
	assert.False(t, c2.IsAborted())
}
