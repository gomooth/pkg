package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/xss"
	"github.com/stretchr/testify/assert"
)

func TestXSSFilter_DefaultStrictPolicy(t *testing.T) {
	r := gin.New()
	r.Use(XSSFilter())
	r.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"<script>alert(1)</script>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "<script>")
}

func TestXSSFilter_SkipRoute(t *testing.T) {
	r := gin.New()
	r.Use(XSSFilter(WithXSSRoutePolicy("/callback", xss.PolicyNone)))
	r.POST("/callback", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/callback", strings.NewReader(`{"data":"<script>alert(1)</script>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// PolicyNone 路由跳过过滤，script 标签应保留
	assert.Contains(t, w.Body.String(), "<script>")
}

func TestXSSFilter_WithUGCPolicy(t *testing.T) {
	r := gin.New()
	r.Use(XSSFilter(WithXSSGlobalPolicy(xss.PolicyUGC)))
	r.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"content":"<b>hello</b><script>alert(1)</script>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// UGC 允许 <b> 标签但过滤 <script>
	assert.NotContains(t, w.Body.String(), "<script>")
}

func TestXSSFilter_WithGlobalSkipFields(t *testing.T) {
	r := gin.New()
	r.Use(XSSFilter(WithXSSGlobalSkipFields("raw_html")))
	r.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"raw_html":"<script>alert(1)</script>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// raw_html 字段应被跳过，保留原始值
	assert.Contains(t, w.Body.String(), "<script>")
}

func TestXSSFilter_QueryString(t *testing.T) {
	r := gin.New()
	r.Use(XSSFilter())
	r.GET("/test", func(c *gin.Context) {
		q := c.Query("q")
		c.String(http.StatusOK, q)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test?q=<script>alert(1)</script>", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "<script>")
}

func TestXSSFilter_FormData(t *testing.T) {
	r := gin.New()
	r.Use(XSSFilter())
	r.POST("/test", func(c *gin.Context) {
		name := c.PostForm("name")
		c.String(http.StatusOK, name)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader("name=<script>alert(1)</script>"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "<script>")
}

func TestXSSFilter_WithTrimSpace(t *testing.T) {
	r := gin.New()
	r.Use(XSSFilter(WithTrimSpaceEnabled(true)))
	r.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"  hello  "}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "hello")
	assert.NotContains(t, w.Body.String(), "  hello  ")
}

func TestXSSFilter_WithMaxBodySize(t *testing.T) {
	t.Run("option creates valid middleware", func(t *testing.T) {
		mw := XSSFilter(WithXSSMaxBodySize(1024))
		assert.NotNil(t, mw)
	})

	t.Run("body exceeds size limit returns 400 without panic", func(t *testing.T) {
		r := gin.New()
		r.Use(XSSFilter(WithXSSMaxBodySize(16))) // 极小限制
		r.POST("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		w := httptest.NewRecorder()
		// 发送超过 16 字节的 body
		body := strings.NewReader(`{"name":"this is definitely longer than 16 bytes"}`)
		req, _ := http.NewRequest(http.MethodPost, "/test", body)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("body within size limit passes normally", func(t *testing.T) {
		r := gin.New()
		r.Use(XSSFilter(WithXSSMaxBodySize(1024)))
		r.POST("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"hello"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestXSSFilter_WithDebug(t *testing.T) {
	r := gin.New()
	r.Use(XSSFilter(WithXSSDebug(true)))
	r.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestXSSFilter_WithRouteFieldPolicy(t *testing.T) {
	r := gin.New()
	r.Use(XSSFilter(
		WithXSSGlobalPolicy(xss.PolicyStrict),
		WithXSSRouteFieldPolicy("/user/", xss.PolicyUGC, "bio"),
	))
	r.POST("/user/profile", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/user/profile", strings.NewReader(`{"bio":"<b>hello</b><script>alert(1)</script>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestXSSFilter_WithGlobalFieldPolicy(t *testing.T) {
	r := gin.New()
	r.Use(XSSFilter(
		WithXSSGlobalPolicy(xss.PolicyNone),
		WithXSSGlobalFieldPolicy(xss.PolicyStrict, "bio"),
	))
	r.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(`{"name":"<script>x</script>","bio":"<script>x</script>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// name 在全局 PolicyNone 下不过滤
	// bio 有专门的字段策略 PolicyStrict，应过滤
}
