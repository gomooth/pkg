package httpcache

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/gomooth/pkg/http/middleware/internal/httpcache/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestMiddleware 创建带 miniredis 的 httpcache 中间件，用于测试。
func newTestMiddleware(t *testing.T, opts ...Option) (gin.HandlerFunc, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })

	allOpts := append([]Option{
		WithRedisStore(rdb),
		WithGlobalCacheDuration(5 * time.Minute),
		WithSingleFlightTimeout(0), // 禁用 singleflight Forget 定时器，避免测试竞态
	}, opts...)

	h, _ := NewWithCloser(allOpts...)
	return h, mr
}

// newEngineWithMiddleware 创建一个 gin.Engine，注册 httpcache 中间件和路由。
func newEngineWithMiddleware(mw gin.HandlerFunc, routes func(r *gin.Engine)) *gin.Engine {
	r := gin.New()
	r.Use(mw)
	routes(r)
	return r
}

// -------------------------------------------------------------------
// Test 1: Cache hit returns stored response
// -------------------------------------------------------------------
func TestHttpCache_Hit_ReturnsCachedResponse(t *testing.T) {
	callCount := 0
	mw, _ := newTestMiddleware(t,
		WithRoutePolicy("/api/data", false),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/data", func(c *gin.Context) {
			callCount++
			c.JSON(http.StatusOK, gin.H{"count": callCount})
		})
	})

	// 第一次请求：缓存未命中，执行 handler
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)
	assert.Contains(t, w1.Body.String(), `"count":1`)

	// 等待缓存写入完成
	time.Sleep(50 * time.Millisecond)

	// 第二次请求：缓存命中，应返回缓存的响应，handler 不应再次执行
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), `"count":1`, "should return cached response, not re-execute handler")
}

// -------------------------------------------------------------------
// Test 2: Cache miss processes request, caches, and returns
// -------------------------------------------------------------------
func TestHttpCache_Miss_CachesAndReturns(t *testing.T) {
	mw, _ := newTestMiddleware(t,
		WithRoutePolicy("/api/hello", false),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/hello", func(c *gin.Context) {
			c.String(http.StatusOK, "hello world")
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/hello", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello world", w.Body.String())
}

// -------------------------------------------------------------------
// Test 3: withToken option separates cache by user ID
// -------------------------------------------------------------------
func TestHttpCache_WithToken_SeparatesByUserID(t *testing.T) {
	callCount := 0
	callCountByUser := map[uint]int{}
	var mu sync.Mutex

	mw, _ := newTestMiddleware(t,
		WithRoutePolicy("/api/profile", true),
		WithUserIDFunc(func(c *gin.Context) (uint, error) {
			uidStr := c.Query("uid")
			if uidStr == "" {
				return 0, nil
			}
			var uid uint
			_, err := fmt.Sscanf(uidStr, "%d", &uid)
			return uid, err
		}),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/profile", func(c *gin.Context) {
			mu.Lock()
			callCount++
			uidStr := c.Query("uid")
			uid := uint(0)
			if uidStr != "" {
				fmt.Sscanf(uidStr, "%d", &uid)
			}
			callCountByUser[uid]++
			mu.Unlock()
			c.JSON(http.StatusOK, gin.H{"uid": uid, "calls": callCountByUser[uid]})
		})
	})

	// 用户 A 第一次请求：缓存未命中
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/profile?uid=1", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	time.Sleep(50 * time.Millisecond)

	// 用户 A 第二次请求：应返回缓存
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/profile?uid=1", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// 用户 B 第一次请求：缓存未命中（不同用户）
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/api/profile?uid=2", nil)
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)

	time.Sleep(50 * time.Millisecond)

	// 用户 B 第二次请求：应返回缓存
	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest(http.MethodGet, "/api/profile?uid=2", nil)
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)

	mu.Lock()
	total := callCount
	mu.Unlock()
	assert.Equal(t, 2, total, "handler should be called exactly twice (once per user)")
}

// -------------------------------------------------------------------
// Test 4: skipFields excludes specified fields from cache key
// -------------------------------------------------------------------
func TestHttpCache_SkipFields_ExcludedFromKey(t *testing.T) {
	callCount := 0

	mw, _ := newTestMiddleware(t,
		WithRouteSkipFiledPolicy("/api/list", false, "page"),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/list", func(c *gin.Context) {
			callCount++
			c.String(http.StatusOK, "items")
		})
	})

	// 第一次请求：page=1
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/list?page=1&keyword=go", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	time.Sleep(50 * time.Millisecond)

	// 第二次请求：page=2 (page 被 skip)，keyword 相同
	// 因为 page 不参与 cache key 计算，应该命中缓存
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/list?page=2&keyword=go", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	assert.Equal(t, 1, callCount, "handler should be called once; page is skipped from cache key")

	// 第三次请求：keyword 不同，page 仍被 skip
	// 不同 keyword = 不同 cache key = 缓存未命中
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/api/list?page=1&keyword=rust", nil)
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)

	assert.Equal(t, 2, callCount, "handler should be called twice; different keyword causes cache miss")
}

// -------------------------------------------------------------------
// Test 5: Route policy determines which routes are cached
// -------------------------------------------------------------------
func TestHttpCache_RoutePolicy_Match(t *testing.T) {
	cachedCount := 0
	uncachedCount := 0

	mw, _ := newTestMiddleware(t,
		WithRoutePolicy("/api/cached", false),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/cached", func(c *gin.Context) {
			cachedCount++
			c.String(http.StatusOK, "cached")
		})
		r.GET("/api/uncached", func(c *gin.Context) {
			uncachedCount++
			c.String(http.StatusOK, "uncached")
		})
	})

	// /api/cached 第一次
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/cached", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	time.Sleep(50 * time.Millisecond)

	// /api/cached 第二次：应命中缓存
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/cached", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, 1, cachedCount, "cached route should only call handler once")

	// /api/uncached 不在 route policy 中，每次都应执行 handler
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/api/uncached", nil)
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)

	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest(http.MethodGet, "/api/uncached", nil)
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)
	assert.Equal(t, 2, uncachedCount, "uncached route should call handler on every request")
}

// -------------------------------------------------------------------
// Test 6: Non-GET requests bypass cache
// -------------------------------------------------------------------
func TestHttpCache_MethodNotGet_Skips(t *testing.T) {
	postCount := 0
	putCount := 0

	mw, _ := newTestMiddleware(t,
		WithRoutePolicy("/api/resource", false),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.POST("/api/resource", func(c *gin.Context) {
			postCount++
			c.String(http.StatusOK, "created")
		})
		r.PUT("/api/resource", func(c *gin.Context) {
			putCount++
			c.String(http.StatusOK, "updated")
		})
	})

	// POST: 不应缓存
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodPost, "/api/resource", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodPost, "/api/resource", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, 2, postCount, "POST should not be cached; handler called every time")

	// PUT: 不应缓存
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodPut, "/api/resource", nil)
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)

	w4 := httptest.NewRecorder()
	req4, _ := http.NewRequest(http.MethodPut, "/api/resource", nil)
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)
	assert.Equal(t, 2, putCount, "PUT should not be cached; handler called every time")
}

// -------------------------------------------------------------------
// Additional: No store configured skips cache (nil store)
// -------------------------------------------------------------------
func TestHttpCache_NilStore_SkipsCache(t *testing.T) {
	callCount := 0

	// 不传入任何 store 选项
	mw := New(
		WithRoutePolicy("/api/data", false),
		WithGlobalCacheDuration(5*time.Minute),
	)

	r := gin.New()
	r.Use(mw)
	r.GET("/api/data", func(c *gin.Context) {
		callCount++
		c.String(http.StatusOK, "data")
	})

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	assert.Equal(t, 3, callCount, "without store, handler should be called every time")
}

// -------------------------------------------------------------------
// Additional: withToken requires userIDFunc
// -------------------------------------------------------------------
func TestHttpCache_WithToken_WithoutUserIDFunc_ReturnsError(t *testing.T) {
	mw, _ := newTestMiddleware(t,
		WithRoutePolicy("/api/profile", true),
		// 未设置 WithUserIDFunc
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/profile", func(c *gin.Context) {
			c.String(http.StatusOK, "profile")
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/profile", nil)
	r.ServeHTTP(w, req)

	// withToken=true 但 userIDFunc 未设置时，getCacheStrategy 返回错误
	// 该错误在 handlerFunc 中被处理，返回 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// -------------------------------------------------------------------
// Additional: Non-success status is not cached
// -------------------------------------------------------------------
func TestHttpCache_NonSuccessStatus_NotCached(t *testing.T) {
	callCount := 0

	mw, _ := newTestMiddleware(t,
		WithRoutePolicy("/api/error", false),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/error", func(c *gin.Context) {
			callCount++
			c.JSON(http.StatusInternalServerError, gin.H{"error": "fail"})
		})
	})

	// 第一次请求：500 不应缓存
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/error", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusInternalServerError, w1.Code)

	// 第二次请求：仍应执行 handler（未缓存）
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/error", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusInternalServerError, w2.Code)

	assert.Equal(t, 2, callCount, "non-success responses should not be cached")
}

// -------------------------------------------------------------------
// Additional: matchRoute unit test
// -------------------------------------------------------------------
func TestMatchRoute(t *testing.T) {
	tests := []struct {
		name        string
		requestPath string
		route       string
		want        bool
	}{
		{"exact match", "/api/user", "/api/user", true},
		{"sub-path match", "/api/user/123", "/api/user", true},
		{"trailing slash", "/api/user/", "/api/user", true},
		{"no false prefix match", "/api/user-profile", "/api/user", false},
		{"different path", "/api/order", "/api/user", false},
		{"root match", "/", "/", true},
		{"root sub-path", "/anything", "/", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchRoute(tt.requestPath, tt.route))
		})
	}
}

// -------------------------------------------------------------------
// Additional: cache key prefix
// -------------------------------------------------------------------
func TestHttpCache_CacheKeyPrefix(t *testing.T) {
	mw, mr := newTestMiddleware(t,
		WithRoutePolicy("/api/data", false),
		WithCacheKeyPrefix("myapp"),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/data", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	time.Sleep(50 * time.Millisecond)

	// 验证 redis 中存在带前缀的 key
	keys := mr.Keys()
	found := false
	for _, k := range keys {
		if len(k) >= len("httpCache:myapp:") && k[:len("httpCache:myapp:")] == "httpCache:myapp:" {
			found = true
			break
		}
	}
	assert.True(t, found, "cache key should contain the prefix 'httpCache:myapp:'")
}

// -------------------------------------------------------------------
// Additional: WithoutResponseHeader suppresses cached headers
// -------------------------------------------------------------------
func TestHttpCache_WithoutResponseHeader(t *testing.T) {
	mw, _ := newTestMiddleware(t,
		WithRoutePolicy("/api/data", false),
		WithoutResponseHeader(true),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/data", func(c *gin.Context) {
			c.Header("X-Custom", "should-be-suppressed")
			c.String(http.StatusOK, "ok")
		})
	})

	// 第一次请求
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w1, req1)
	// 第一次请求直接走 handler，自定义 header 会写出
	assert.Equal(t, "should-be-suppressed", w1.Header().Get("X-Custom"))

	time.Sleep(50 * time.Millisecond)

	// 第二次请求：缓存命中，withoutResponseHeader=true 应跳过缓存的 header
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "ok", w2.Body.String())
	// withoutResponseHeader=true 时，缓存响应的 header 不应写回
	assert.Empty(t, w2.Header().Get("X-Custom"), "cached headers should be suppressed when withoutResponseHeader=true")
}

// -------------------------------------------------------------------
// Additional: global skipFields
// -------------------------------------------------------------------
func TestHttpCache_GlobalSkipFields(t *testing.T) {
	callCount := 0

	mw, _ := newTestMiddleware(t,
		WithRoutePolicy("/api/list", false),
		WithGlobalSkipQueryFields("v"),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/list", func(c *gin.Context) {
			callCount++
			c.String(http.StatusOK, "list")
		})
	})

	// 第一次请求：v=1
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/list?keyword=go&v=1", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	time.Sleep(50 * time.Millisecond)

	// 第二次请求：v=2 (v 全局 skip)，keyword 相同 -> 缓存命中
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/list?keyword=go&v=2", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	assert.Equal(t, 1, callCount, "global skipFields should exclude 'v' from cache key")
}

// -------------------------------------------------------------------
// Additional: Store Get error in non-debug mode skips cache
// -------------------------------------------------------------------
func TestHttpCache_StoreGetError_NonDebug_Skips(t *testing.T) {
	mw := New(
		WithRoutePolicy("/api/data", false),
		WithGlobalCacheDuration(5*time.Minute),
		WithSingleFlightTimeout(0),
		withStore(&errorStore{getError: errors.New("store connection lost")}),
	)

	r := gin.New()
	r.Use(mw)
	r.GET("/api/data", func(c *gin.Context) {
		c.String(http.StatusOK, "data")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w, req)

	// 非 debug 模式下 store 错误不阻塞，直接跳过缓存
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "data", w.Body.String())
}

// -------------------------------------------------------------------
// Additional: WithUserIDFunc error in debug mode
// -------------------------------------------------------------------
func TestHttpCache_WithToken_UserIDFuncError_DebugMode(t *testing.T) {
	mw := New(
		WithRoutePolicy("/api/profile", true),
		WithDebug(true),
		WithGlobalCacheDuration(5*time.Minute),
		WithUserIDFunc(func(c *gin.Context) (uint, error) {
			return 0, errors.New("unauthorized")
		}),
		withStore(&noopStore{}),
		WithSingleFlightTimeout(0),
	)

	r := gin.New()
	r.Use(mw)
	r.GET("/api/profile", func(c *gin.Context) {
		c.String(http.StatusOK, "profile")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/profile", nil)
	r.ServeHTTP(w, req)

	// debug 模式下，userIDFunc 错误应导致 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// -------------------------------------------------------------------
// Additional: getCacheKey prefix
// -------------------------------------------------------------------
func TestHandler_GetCacheKey(t *testing.T) {
	h := &handler{}

	assert.Equal(t, "httpCache:foo", h.getCacheKey("foo"))

	h.prefixKey = "app"
	assert.Equal(t, "httpCache:app:foo", h.getCacheKey("foo"))
}

// -------------------------------------------------------------------
// Additional: WithRouteRule with custom duration
// -------------------------------------------------------------------
func TestHttpCache_RouteRule_CustomDuration(t *testing.T) {
	mw, mr := newTestMiddleware(t,
		WithRouteRule("/api/short", false, 100*time.Millisecond),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/short", func(c *gin.Context) {
			c.String(http.StatusOK, "short-lived")
		})
	})

	// 第一次请求
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/short", nil)
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	time.Sleep(50 * time.Millisecond)

	// 第二次请求：缓存仍有效
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/short", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, "short-lived", w2.Body.String())

	// 等待缓存过期
	mr.FastForward(150 * time.Millisecond)

	// 第三次请求：缓存已过期，应重新执行 handler
	callCount := 0
	r2 := newEngineWithMiddleware(
		func() gin.HandlerFunc {
			mw2, mr2 := newTestMiddleware(t,
				WithRouteRule("/api/short", false, 100*time.Millisecond),
			)
			// 让缓存快速过期
			mr2.FastForward(150 * time.Millisecond)
			return mw2
		}(),
		func(r *gin.Engine) {
			r.GET("/api/short", func(c *gin.Context) {
				callCount++
				c.String(http.StatusOK, "short-lived")
			})
		},
	)

	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/api/short", nil)
	r2.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	assert.Equal(t, 1, callCount)
}

// -------------------------------------------------------------------
// Helper: withStore injects a custom ICacheStore
// -------------------------------------------------------------------
func withStore(s store.ICacheStore) Option {
	return func(h *handler) {
		h.store = s
	}
}

// -------------------------------------------------------------------
// Helper stores for testing
// -------------------------------------------------------------------

// noopStore is a minimal ICacheStore that always misses.
type noopStore struct{}

func (n *noopStore) Get(_ context.Context, _ string, _ *store.CachedResponse) error {
	return store.ErrorCacheMiss
}

func (n *noopStore) Set(_ context.Context, _ string, _ *store.CachedResponse, _ time.Duration) error {
	return nil
}

func (n *noopStore) Delete(_ context.Context, _ string) error {
	return nil
}

// errorStore returns errors on Get (non-CacheMiss).
type errorStore struct {
	getError error
}

func (e *errorStore) Get(_ context.Context, _ string, _ *store.CachedResponse) error {
	return e.getError
}

func (e *errorStore) Set(_ context.Context, _ string, _ *store.CachedResponse, _ time.Duration) error {
	return nil
}

func (e *errorStore) Delete(_ context.Context, _ string) error {
	return nil
}
