package httpcache

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"github.com/gomooth/pkg/http/middleware/internal/httpcache/store"
)

// TestWithRedisStoreBy 通过地址创建 Redis store
func TestWithRedisStoreBy(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	opt := WithRedisStoreBy(mr.Addr(), 0)
	h := &handler{}
	opt(h)

	assert.NotNil(t, h.store)
}

// TestWithRedisStoreBy_EmptyAddr 空地址不设置 store
func TestWithRedisStoreBy_EmptyAddr(t *testing.T) {
	opt := WithRedisStoreBy("", 0)
	h := &handler{}
	opt(h)

	assert.Nil(t, h.store)
}

// TestWithRedisStore_NilClient nil client 不设置 store
func TestWithRedisStore_NilClient(t *testing.T) {
	opt := WithRedisStore(nil)
	h := &handler{}
	opt(h)

	assert.Nil(t, h.store)
}

// TestWithLogger 设置自定义 logger
func TestWithLogger(t *testing.T) {
	log := slog.Default()
	opt := WithLogger(log)
	h := &handler{}
	opt(h)

	assert.Equal(t, log, h.log)
}

// TestWithGlobalHeaderKey 设置全局 header key
func TestWithGlobalHeaderKey(t *testing.T) {
	opt := WithGlobalHeaderKey([]string{"X-Request-ID", "Accept-Language"})
	h := &handler{}
	opt(h)

	assert.Equal(t, []string{"X-Request-ID", "Accept-Language"}, h.globalHeaderKeys)
}

// TestWithAppendGlobalHeaderKey_EmptyInitial 追加全局 header key（初始为空）
func TestWithAppendGlobalHeaderKey_EmptyInitial(t *testing.T) {
	opt := WithAppendGlobalHeaderKey("X-Request-ID")
	h := &handler{}
	opt(h)

	assert.Equal(t, []string{"X-Request-ID"}, h.globalHeaderKeys)
}

// TestWithAppendGlobalHeaderKey_ExistingValues 追加到已有值
func TestWithAppendGlobalHeaderKey_ExistingValues(t *testing.T) {
	opt := WithAppendGlobalHeaderKey("X-New-Header")
	h := &handler{
		globalHeaderKeys: []string{"X-Existing"},
	}
	opt(h)

	assert.Contains(t, h.globalHeaderKeys, "X-Existing")
	assert.Contains(t, h.globalHeaderKeys, "X-New-Header")
}

// TestWithRouteSkipFiledRule 带忽略字段和自定义时长的路由规则
func TestWithRouteSkipFiledRule(t *testing.T) {
	opt := WithRouteSkipFiledRule("/api/list", false, 5*time.Minute, "page", "size")
	h := &handler{
		routeList:     make([]string, 0),
		routePolicies: make(map[string]*ruleItem),
	}
	opt(h)

	assert.Contains(t, h.routeList, "/api/list")
	rule := h.routePolicies["/api/list"]
	assert.NotNil(t, rule)
	assert.False(t, rule.withToken)
	assert.Equal(t, 5*time.Minute, rule.duration)
	assert.Contains(t, rule.skipFields, "page")
	assert.Contains(t, rule.skipFields, "size")
}

// TestNewWithCloser 返回 handler 和 closer
func TestNewWithCloser(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	mw, closer := NewWithCloser(
		WithRedisStore(rdb),
		WithRoutePolicy("/api/test", false),
	)
	assert.NotNil(t, mw)
	assert.NotNil(t, closer)

	// closer 应该正常执行（不拥有 client 时为空操作）
	err := closer()
	assert.NoError(t, err)
}

// TestNewWithCloser_OwnedRedisStore closer 关闭拥有的 Redis 连接
func TestNewWithCloser_OwnedRedisStore(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()

	mw, closer := NewWithCloser(
		WithRedisStoreBy(mr.Addr(), 0),
		WithRoutePolicy("/api/test", false),
	)
	assert.NotNil(t, mw)

	err := closer()
	assert.NoError(t, err)
}

// TestDebugf_DebugModeWithLogger debug 模式 + 自定义 logger
func TestDebugf_DebugModeWithLogger(t *testing.T) {
	h := &handler{
		debug: true,
		log:   slog.Default(),
	}
	// 不应 panic
	h.debugf("test message: %s", "arg")
}

// TestDebugf_DebugModeWithoutLogger debug 模式 + 默认 slog
func TestDebugf_DebugModeWithoutLogger(t *testing.T) {
	h := &handler{
		debug: true,
		log:   nil,
	}
	// 不应 panic
	h.debugf("test message: %s", "arg")
}

// TestDebugf_NonDebugMode 非 debug 模式不输出
func TestDebugf_NonDebugMode(t *testing.T) {
	h := &handler{
		debug: false,
	}
	// 不应 panic
	h.debugf("test message: %s", "arg")
}

// TestCached_StoreSetError_NonDebugMode 非 debug 模式下 store Set 错误不阻塞
func TestCached_StoreSetError_NonDebugMode(t *testing.T) {
	mw := New(
		WithRoutePolicy("/api/data", false),
		WithDebug(false),
		WithGlobalCacheDuration(5*time.Minute),
		WithSingleFlightTimeout(0),
		withStore(&setFailStore{}),
	)

	r := gin.New()
	r.Use(mw)
	r.GET("/api/data", func(c *gin.Context) {
		c.String(http.StatusOK, "data")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	r.ServeHTTP(w, req)

	// 非 debug 模式下 store Set 错误不阻塞，返回正常响应
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestWithRoutePolicy_WithFields 带 fields 的路由策略
func TestWithRoutePolicy_WithFields(t *testing.T) {
	opt := WithRoutePolicy("/api/search", false, "keyword", "category")
	h := &handler{
		routeList:     make([]string, 0),
		routePolicies: make(map[string]*ruleItem),
	}
	opt(h)

	rule := h.routePolicies["/api/search"]
	assert.NotNil(t, rule)
	assert.Contains(t, rule.fields, "keyword")
	assert.Contains(t, rule.fields, "category")
}

// TestWithRouteRule_WithHeaderKeys 带 headerKeys 的路由规则
func TestWithRouteRule_WithHeaderKeys(t *testing.T) {
	opt := withRouteRule("/api/data", false, 5*time.Minute, nil, []string{"X-Request-ID"}, nil)
	h := &handler{
		routeList:     make([]string, 0),
		routePolicies: make(map[string]*ruleItem),
	}
	opt(h)

	rule := h.routePolicies["/api/data"]
	assert.NotNil(t, rule)
	assert.Contains(t, rule.headerKeys, "X-Request-ID")
}

// TestWithRouteRule_DuplicateRoute 重复路由覆盖
func TestWithRouteRule_DuplicateRoute(t *testing.T) {
	h := &handler{
		routeList:     []string{"/api/data"},
		routePolicies: map[string]*ruleItem{"/api/data": {withToken: false}},
	}

	opt1 := WithRoutePolicy("/api/data", true)
	opt1(h)

	rule := h.routePolicies["/api/data"]
	assert.True(t, rule.withToken, "should update existing rule")
}

// TestResponseWriter_WriteString 自定义 ResponseWriter 的 WriteString 方法
func TestResponseWriter_WriteString(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = &http.Request{Header: make(http.Header)}
	rw := newResponseWriter(c.Writer)

	n, err := rw.WriteString("hello")
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", rw.body.String())
}

// TestRuleItem_String ruleItem 的 String 方法
func TestRuleItem_String(t *testing.T) {
	r := &ruleItem{
		withToken:  true,
		fields:     map[string]struct{}{"keyword": {}},
		skipFields: map[string]struct{}{"page": {}},
		duration:   5 * time.Minute,
	}

	s := r.String()
	assert.Contains(t, s, "withToken")
	assert.Contains(t, s, "keyword")
	assert.Contains(t, s, "page")
}

// TestHttpCache_WithGlobalHeaderKeys_Integration 集成测试：全局 header key 参与缓存 key 计算
func TestHttpCache_WithGlobalHeaderKeys_Integration(t *testing.T) {
	callCount := 0
	mw, _ := newTestMiddleware(t,
		WithRoutePolicy("/api/data", false),
		WithGlobalHeaderKey([]string{"Accept-Language"}),
	)

	r := newEngineWithMiddleware(mw, func(r *gin.Engine) {
		r.GET("/api/data", func(c *gin.Context) {
			callCount++
			c.String(http.StatusOK, "data")
		})
	})

	// 第一次请求：Accept-Language=zh-CN
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	req1.Header.Set("Accept-Language", "zh-CN")
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	time.Sleep(50 * time.Millisecond)

	// 第二次请求：相同 Accept-Language 应命中缓存
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	req2.Header.Set("Accept-Language", "zh-CN")
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Equal(t, 1, callCount)

	// 第三次请求：不同 Accept-Language 应缓存未命中
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/api/data", nil)
	req3.Header.Set("Accept-Language", "en-US")
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	assert.Equal(t, 2, callCount)
}

// TestHttpCache_WithToken_UserIDFuncError 非 debug 模式下 userIDFunc 错误返回 500
func TestHttpCache_WithToken_UserIDFuncError_NonDebug(t *testing.T) {
	mw := New(
		WithRoutePolicy("/api/profile", true),
		WithDebug(false),
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

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- 辅助类型 ---

// setFailStore Get 返回 CacheMiss，Set 返回错误
type setFailStore struct{}

func (s *setFailStore) Get(_ context.Context, _ string, _ *store.CachedResponse) error {
	return store.ErrorCacheMiss
}

func (s *setFailStore) Set(_ context.Context, _ string, _ *store.CachedResponse, _ time.Duration) error {
	return errors.New("redis set failed")
}

func (s *setFailStore) Delete(_ context.Context, _ string) error {
	return nil
}
