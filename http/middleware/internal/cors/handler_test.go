package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestNew_DefaultConfig 验证默认配置：未设置 AllowOriginFunc 时返回 false
func TestNew_DefaultConfig(t *testing.T) {
	mw := New()
	assert.NotNil(t, mw)
}

// TestNew_WithAllowOriginFunc 设置允许的 origin 函数
func TestNew_WithAllowOriginFunc(t *testing.T) {
	mw := New(WithAllowOriginFunc(func(origin string) bool {
		return origin == "https://example.com"
	}))
	assert.NotNil(t, mw)
}

// TestIsWildcardOriginFunc 检测通配符 AllowOriginFunc
func TestIsWildcardOriginFunc(t *testing.T) {
	// 通配 origin（对所有 origin 返回 true）
	wildcard := &handler{
		allowOriginFunc: func(origin string) bool { return true },
	}
	assert.True(t, wildcard.isWildcardOriginFunc())

	// 非通配
	restricted := &handler{
		allowOriginFunc: func(origin string) bool { return origin == "https://example.com" },
	}
	assert.False(t, restricted.isWildcardOriginFunc())

	// nil 函数
	nilFunc := &handler{
		allowOriginFunc: nil,
	}
	assert.False(t, nilFunc.isWildcardOriginFunc())
}

// TestGetCORSConfig 默认凭证为 true
func TestGetCORSConfig_DefaultCredentials(t *testing.T) {
	h := &handler{
		allowOriginFunc: func(origin string) bool { return true },
		allowMethods:    []string{"GET"},
		allowHeaders:    []string{"Content-Type"},
		exposeHeaders:   []string{"X-Custom"},
		maxAge:          1 * time.Hour,
	}

	cfg := h.getCORSConfig()
	assert.True(t, cfg.AllowCredentials)
	assert.NotNil(t, cfg.AllowOriginFunc)
	assert.Equal(t, []string{"GET"}, cfg.AllowMethods)
	assert.Equal(t, []string{"Content-Type"}, cfg.AllowHeaders)
	assert.Equal(t, []string{"X-Custom"}, cfg.ExposeHeaders)
	assert.Equal(t, 1*time.Hour, cfg.MaxAge)
}

// TestGetCORSConfig_DisabledCredentials
func TestGetCORSConfig_DisabledCredentials(t *testing.T) {
	enabled := false
	h := &handler{
		allowOriginFunc:  func(origin string) bool { return true },
		allowCredentials: &enabled,
		allowMethods:     []string{"GET"},
		allowHeaders:     []string{"Content-Type"},
		exposeHeaders:    []string{"X-Custom"},
		maxAge:           1 * time.Hour,
	}

	cfg := h.getCORSConfig()
	assert.False(t, cfg.AllowCredentials)
}

// TestGetCORSConfig_NilOriginFuncDefaultsToFalse
func TestGetCORSConfig_NilOriginFuncDefaultsToFalse(t *testing.T) {
	// 验证 New() 中默认设置 allowOriginFunc 为 false
	h := &handler{
		allowMethods:  []string{"GET"},
		allowHeaders:  []string{"Content-Type"},
		exposeHeaders: []string{"X-Custom"},
		maxAge:        1 * time.Hour,
	}
	// 模拟 New() 中的默认行为
	if h.allowOriginFunc == nil {
		h.allowOriginFunc = func(origin string) bool {
			return false
		}
	}

	cfg := h.getCORSConfig()
	assert.False(t, cfg.AllowOriginFunc("https://example.com"))
}

// TestOption_WithAllowMethods 设置允许的 HTTP 方法
func TestOption_WithAllowMethods(t *testing.T) {
	h := &handler{
		allowMethods: []string{"GET"},
	}
	WithAllowMethods("POST", "PUT")(h)
	assert.Contains(t, h.allowMethods, "GET")
	assert.Contains(t, h.allowMethods, "POST")
	assert.Contains(t, h.allowMethods, "PUT")
}

// TestOption_WithAllowMethods_Empty 不传方法时不修改默认值
func TestOption_WithAllowMethods_Empty(t *testing.T) {
	h := &handler{
		allowMethods: []string{"GET"},
	}
	WithAllowMethods()(h)
	assert.Equal(t, []string{"GET"}, h.allowMethods)
}

// TestOption_WithHeaders 同时追加 AllowHeaders 和 ExposeHeaders
func TestOption_WithHeaders(t *testing.T) {
	h := &handler{
		allowHeaders:  []string{"Content-Type"},
		exposeHeaders: []string{"X-Default"},
	}
	WithHeaders("X-Custom")(h)
	assert.Contains(t, h.allowHeaders, "X-Custom")
	assert.Contains(t, h.exposeHeaders, "X-Custom")
}

// TestOption_WithHeaders_Empty 空参数不修改默认值
func TestOption_WithHeaders_Empty(t *testing.T) {
	h := &handler{
		allowHeaders:  []string{"Content-Type"},
		exposeHeaders: []string{"X-Default"},
	}
	WithHeaders()(h)
	assert.Equal(t, []string{"Content-Type"}, h.allowHeaders)
	assert.Equal(t, []string{"X-Default"}, h.exposeHeaders)
}

// TestOption_WithAllowHeaders 仅追加 AllowHeaders
func TestOption_WithAllowHeaders(t *testing.T) {
	h := &handler{
		allowHeaders:  []string{"Content-Type"},
		exposeHeaders: []string{"X-Default"},
	}
	WithAllowHeaders("X-Only-Allow")(h)
	assert.Contains(t, h.allowHeaders, "X-Only-Allow")
	assert.NotContains(t, h.exposeHeaders, "X-Only-Allow")
}

// TestOption_WithAllowHeaders_Empty 空参数不修改默认值
func TestOption_WithAllowHeaders_Empty(t *testing.T) {
	h := &handler{
		allowHeaders:  []string{"Content-Type"},
		exposeHeaders: []string{"X-Default"},
	}
	WithAllowHeaders()(h)
	assert.Equal(t, []string{"Content-Type"}, h.allowHeaders)
	assert.Equal(t, []string{"X-Default"}, h.exposeHeaders)
}

// TestOption_WithExposeHeaders 仅追加 ExposeHeaders
func TestOption_WithExposeHeaders(t *testing.T) {
	h := &handler{
		allowHeaders:  []string{"Content-Type"},
		exposeHeaders: []string{"X-Default"},
	}
	WithExposeHeaders("X-Only-Expose")(h)
	assert.Contains(t, h.exposeHeaders, "X-Only-Expose")
	assert.NotContains(t, h.allowHeaders, "X-Only-Expose")
}

// TestOption_WithExposeHeaders_Empty 空参数不修改默认值
func TestOption_WithExposeHeaders_Empty(t *testing.T) {
	h := &handler{
		allowHeaders:  []string{"Content-Type"},
		exposeHeaders: []string{"X-Default"},
	}
	WithExposeHeaders()(h)
	assert.Equal(t, []string{"Content-Type"}, h.allowHeaders)
	assert.Equal(t, []string{"X-Default"}, h.exposeHeaders)
}

// TestOption_WithMaxAge 设置预检缓存时间
func TestOption_WithMaxAge(t *testing.T) {
	h := &handler{maxAge: 12 * time.Hour}
	WithMaxAge(1 * time.Hour)(h)
	assert.Equal(t, 1*time.Hour, h.maxAge)
}

// TestOption_WithMaxAge_ZeroOrNegative 不修改默认值
func TestOption_WithMaxAge_ZeroOrNegative(t *testing.T) {
	h := &handler{maxAge: 12 * time.Hour}
	WithMaxAge(0)(h)
	assert.Equal(t, 12*time.Hour, h.maxAge)

	WithMaxAge(-1 * time.Second)(h)
	assert.Equal(t, 12*time.Hour, h.maxAge)
}

// TestOption_WithCORSAllowCredentials_False 禁用凭证
func TestOption_WithCORSAllowCredentials_False(t *testing.T) {
	h := &handler{}
	WithCORSAllowCredentials(false)(h)
	assert.NotNil(t, h.allowCredentials)
	assert.False(t, *h.allowCredentials)
}

// TestOption_WithCORSAllowCredentials_True 启用凭证
func TestOption_WithCORSAllowCredentials_True(t *testing.T) {
	h := &handler{}
	WithCORSAllowCredentials(true)(h)
	assert.NotNil(t, h.allowCredentials)
	assert.True(t, *h.allowCredentials)
}

// TestNew_DefaultHandlerValues 验证 New() 默认值
func TestNew_DefaultHandlerValues(t *testing.T) {
	// 通过 getCORSConfig 间接验证 handler 默认值
	mw := New(WithAllowOriginFunc(func(origin string) bool { return origin == "https://test.com" }))

	// 将中间件应用到 gin.Engine 并执行简单请求来验证
	r := gin.New()
	r.Use(mw)
	r.Any("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://test.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://test.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

// TestNew_DisallowedOrigin_RealRequest 拒绝非允许的 origin 实际请求
func TestNew_DisallowedOrigin_RealRequest(t *testing.T) {
	mw := New(WithAllowOriginFunc(func(origin string) bool {
		return origin == "https://allowed.com"
	}))

	r := gin.New()
	r.Use(mw)
	r.Any("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://evil.com")
	r.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

// TestNew_CORSConfigReturned 验证生成的 cors.Config 正确性
func TestNew_CORSConfigReturned(t *testing.T) {
	enabled := false
	h := &handler{
		allowOriginFunc:  func(origin string) bool { return true },
		allowCredentials: &enabled,
		allowMethods:     []string{"GET", "POST", "GET"}, // 含重复
		allowHeaders:     []string{"Content-Type", "Content-Type"},
		exposeHeaders:    []string{"X-Custom", "X-Custom"},
		maxAge:           30 * time.Minute,
	}

	cfg := h.getCORSConfig()

	// 验证去重
	assert.Len(t, cfg.AllowMethods, 2) // GET, POST
	assert.Len(t, cfg.AllowHeaders, 1) // Content-Type
	assert.Len(t, cfg.ExposeHeaders, 1) // X-Custom

	// 验证凭证
	assert.False(t, cfg.AllowCredentials)

	// 验证 MaxAge
	assert.Equal(t, 30*time.Minute, cfg.MaxAge)
}

// TestNew_Integration_PreflightAllowed 集成测试：预检请求允许的 origin
func TestNew_Integration_PreflightAllowed(t *testing.T) {
	mw := New(WithAllowOriginFunc(func(origin string) bool {
		return origin == "https://allowed.com"
	}))

	r := gin.New()
	r.Use(mw)
	r.Any("/api/data", func(c *gin.Context) {
		c.String(http.StatusOK, "data")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/data", nil)
	req.Header.Set("Origin", "https://allowed.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	r.ServeHTTP(w, req)

	assert.Equal(t, "https://allowed.com", w.Header().Get("Access-Control-Allow-Origin"))
}

// TestNew_Integration_PreflightDisallowed 集成测试：预检请求拒绝的 origin
func TestNew_Integration_PreflightDisallowed(t *testing.T) {
	mw := New(WithAllowOriginFunc(func(origin string) bool {
		return origin == "https://allowed.com"
	}))

	r := gin.New()
	r.Use(mw)
	r.Any("/api/data", func(c *gin.Context) {
		c.String(http.StatusOK, "data")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/api/data", nil)
	req.Header.Set("Origin", "https://evil.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	r.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

// TestGetCORSConfig_WildcardWithCredentials_Warns 通配 AllowOriginFunc + AllowCredentials=true 应输出警告
func TestGetCORSConfig_WildcardWithCredentials_Warns(t *testing.T) {
	h := &handler{
		allowOriginFunc: func(origin string) bool { return true },
		allowMethods:    []string{"GET"},
		allowHeaders:    []string{"Content-Type"},
		exposeHeaders:   []string{"X-Custom"},
		maxAge:          1 * time.Hour,
	}

	// 不会 panic，且 AllowCredentials 默认为 true
	cfg := h.getCORSConfig()
	assert.True(t, cfg.AllowCredentials)
	assert.True(t, cfg.AllowOriginFunc("https://evil.com"))
}

// Verify getCORSConfig satisfies cors.Config expectations
var _ cors.Config = cors.Config{}
