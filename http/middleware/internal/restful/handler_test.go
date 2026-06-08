package restful

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestGinContext 创建带 httpcontext 的 gin.Context
func newRestfulTestGinContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
	}
	// 初始化 httpcontext
	stx := httpcontext.NewContext()
	stx.StorageTo(c)
	return c
}

// mockRole 实现 IRole
type rMockRole string

func (r rMockRole) String() string { return string(r) }

// TestHandle_DefaultAccept 空 Accept 使用默认版本
func TestHandle_DefaultAccept(t *testing.T) {
	c := newRestfulTestGinContext()
	c.Request.Header.Set("Accept", "")

	h := New(c, "v1")
	err := h.Handle()
	assert.NoError(t, err)

	stx, _ := httpcontext.Parse(c)
	assert.Equal(t, "v1", stx.Value("version"))
	assert.Equal(t, "raw", stx.Value("bodyProperty"))
}

// TestHandle_WildcardAccept 通配 Accept 使用默认版本
func TestHandle_WildcardAccept(t *testing.T) {
	c := newRestfulTestGinContext()
	c.Request.Header.Set("Accept", "*/*")

	h := New(c, "v2")
	err := h.Handle()
	assert.NoError(t, err)

	stx, _ := httpcontext.Parse(c)
	assert.Equal(t, "v2", stx.Value("version"))
}

// TestHandle_ApplicationJSON Accept 为 application/json 使用默认版本
func TestHandle_ApplicationJSON(t *testing.T) {
	c := newRestfulTestGinContext()
	c.Request.Header.Set("Accept", "application/json")

	h := New(c, "v1")
	err := h.Handle()
	assert.NoError(t, err)

	stx, _ := httpcontext.Parse(c)
	assert.Equal(t, "v1", stx.Value("version"))
	assert.Equal(t, "raw", stx.Value("bodyProperty"))
}

// TestHandle_ValidCustomMediaType 有效的自定义媒体类型
func TestHandle_ValidCustomMediaType(t *testing.T) {
	tests := []struct {
		name           string
		accept         string
		wantVersion    string
		wantBodyProp   string
	}{
		{"v1 raw", "application/vnd.server.v1.raw+json", "v1", "raw"},
		{"v2 text", "application/vnd.server.v2.text+json", "v2", "text"},
		{"v3 html", "application/vnd.server.v3.html+json", "v3", "html"},
		{"v1 full", "application/vnd.server.v1.full+json", "v1", "full"},
		{"v1.0.0 raw", "application/vnd.server.v1.0.0.raw+json", "v1.0.0", "raw"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newRestfulTestGinContext()
			c.Request.Header.Set("Accept", tt.accept)

			h := New(c, "default")
			err := h.Handle()
			assert.NoError(t, err)

			stx, _ := httpcontext.Parse(c)
			assert.Equal(t, tt.wantVersion, stx.Value("version"))
			assert.Equal(t, tt.wantBodyProp, stx.Value("bodyProperty"))
		})
	}
}

// TestHandle_CustomMediaType_NoBodyProperty 省略 bodyProperty 默认为 raw
func TestHandle_CustomMediaType_NoBodyProperty(t *testing.T) {
	c := newRestfulTestGinContext()
	// 不带 bodyProperty 部分的自定义媒体类型
	c.Request.Header.Set("Accept", "application/vnd.server.v1+json")

	h := New(c, "default")
	err := h.Handle()
	// 正则不匹配（需要 5 个参数），应返回错误
	assert.NotNil(t, err)
}

// TestHandle_InvalidVersion 版本号格式无效（v 开头但非 semver）
func TestHandle_InvalidVersion(t *testing.T) {
	c := newRestfulTestGinContext()
	c.Request.Header.Set("Accept", "application/vnd.server.v2.x.raw+json")

	h := New(c, "default")
	err := h.Handle()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not support api version")
}

// TestHandle_EmptyBodyProperty 正则匹配但 bodyProperty 为空时返回错误
func TestHandle_EmptyBodyProperty(t *testing.T) {
	c := newRestfulTestGinContext()
	// 正则匹配（len==5），但 bodyProperty 为空
	c.Request.Header.Set("Accept", "application/vnd.server.v1+json")

	h := New(c, "default")
	err := h.Handle()
	// params[4] 为空，不属于 raw/text/html/full
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not support custom media type")
}

// TestHandle_UnsupportedMediaType 完全不支持的自定义媒体类型
func TestHandle_UnsupportedMediaType(t *testing.T) {
	c := newRestfulTestGinContext()
	c.Request.Header.Set("Accept", "text/html")

	h := New(c, "default")
	err := h.Handle()
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "not support custom media type")
}

// TestHandle_NoHttpContext gin.Context 中没有 httpcontext 返回错误
func TestHandle_NoHttpContext(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
	}
	// 不设置 httpcontext

	h := New(c, "v1")
	err := h.Handle()
	assert.NotNil(t, err)
}

// TestNew 构造函数
func TestNew(t *testing.T) {
	c := newRestfulTestGinContext()
	h := New(c, "v1")
	assert.NotNil(t, h)
	assert.Equal(t, c, h.ctx)
	assert.Equal(t, "v1", h.version)
}
