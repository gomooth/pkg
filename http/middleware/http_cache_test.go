package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestHttpCache_FirstMissSecondHit(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	var callCount atomic.Int32

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheRouteRule("/api/data", false, 5*time.Minute),
	))
	r.GET("/api/data", func(c *gin.Context) {
		callCount.Add(1)
		c.JSON(http.StatusOK, gin.H{"count": callCount.Load()})
	})

	// 第一次请求 miss
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, int32(1), callCount.Load())

	// 第二次请求 hit
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	// 命中缓存时不会再次调用 handler
	assert.Equal(t, int32(1), callCount.Load())
}

func TestHttpCache_SkipFieldNotAffectCacheKey(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	var callCount atomic.Int32

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheRouteSkipFiledPolicy("/api/list", false, "page"),
	))
	r.GET("/api/list", func(c *gin.Context) {
		callCount.Add(1)
		c.JSON(http.StatusOK, gin.H{"items": []string{}})
	})

	// 第一次请求 page=1
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/list?page=1", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Equal(t, int32(1), callCount.Load())

	// 第二次请求 page=2，但 page 是忽略字段，应命中同一缓存
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/list?page=2", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, int32(1), callCount.Load())
}

func TestHttpCache_NoStore_Passthrough(t *testing.T) {
	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache())
	r.GET("/api/data", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
