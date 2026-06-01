package middleware

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestWithHttpCacheLogger(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	logger := slog.Default()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheLogger(logger),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithHttpCacheDebug(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheDebug(true),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithHttpCacheUserIDFunc(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheUserIDFunc(func(c *gin.Context) (uint, error) {
			return 1, nil
		}),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithHttpCacheGlobalHeaderKeys(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheGlobalHeaderKeys([]string{"X-Request-ID"}),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithHttpCacheGlobalHeaderKey(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheGlobalHeaderKey("X-Custom-Key"),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithHttpCacheGlobalSkipFields(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheGlobalSkipFields("v"),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithHttpCacheKeyPrefix(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheKeyPrefix("myapp:"),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithoutHttpCacheResponseHeader(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithoutHttpCacheResponseHeader(true),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithHttpCacheRoutePolicy(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheRoutePolicy("/api/data", false),
	))
	r.GET("/api/data", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithHttpCacheRouteSkipFiledRule(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
		WithHttpCacheRouteSkipFiledRule("/api/list", false, 5*time.Minute, "page"),
	))
	r.GET("/api/list", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/list", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestWithHttpCacheRedisStoreBy(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	r := gin.New()
	r.Use(HttpContext())
	r.Use(HttpCache(
		WithHttpCacheRedisStoreBy(mr.Addr(), 0),
		WithHttpCacheGlobalDuration(5*time.Minute),
	))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHttpCacheWithCloser(t *testing.T) {
	mr, err := miniredis.Run()
	assert.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	mw, closer := HttpCacheWithCloser(
		WithHttpCacheRedisStore(rdb),
		WithHttpCacheGlobalDuration(5*time.Minute),
	)
	assert.NotNil(t, mw)
	assert.NotNil(t, closer)

	r := gin.New()
	r.Use(HttpContext())
	r.Use(mw)
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
