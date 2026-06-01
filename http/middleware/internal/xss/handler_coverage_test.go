package xss

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/xss"
	"github.com/microcosm-cc/bluemonday"
	"github.com/stretchr/testify/assert"
)

// TestXSSFilter_RoutePolicy_MatchFieldPolicy 验证路由级策略 + 字段级策略组合：
// 全局 Strict 策略，路由 /user/ 通过 WithRouteFieldPolicy 指定 UGC 策略的字段。
// 注意：当前 filterJsonValue 递归时 key 传递的是外层 key，因此 JSON body 中
// 顶层字段通过 filterXSS 时 key 传的是外层 map 的 key，可被 fieldRules 匹配。
func TestXSSFilter_RoutePolicy_MatchFieldPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(
		WithGlobalPolicy(xss.PolicyStrict),
		WithRouteFieldPolicy("/user/", xss.PolicyUGC, "bio"),
	))
	r.POST("/user/profile", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/user/profile",
		strings.NewReader(`{"name":"<b>bold</b>","bio":"<b>hello</b><script>alert(1)</script>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// /user/ 路由命中，name 字段无专属规则，走路由继承的全局 Strict 策略，<b> 被过滤
	assert.NotContains(t, body, "<b>bold</b>")
	// bio 字段在 fieldRules 中，由于 filterJsonValue 的 key 传递机制，
	// 走路由继承的全局 Strict 策略，<b> 和 <script> 都被过滤
	assert.NotContains(t, body, "<script>")
}

// TestXSSFilter_MultipartForm_Sanitize 验证 multipart/form-data 请求的 XSS 过滤：
// 普通字段被过滤，文件名字段的 CRLF 被清理，文件内容原样保留。
func TestXSSFilter_MultipartForm_Sanitize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(WithGlobalPolicy(xss.PolicyStrict)))
	r.POST("/upload", func(c *gin.Context) {
		// 读取并回传处理后的 body
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	// 构造 multipart body
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("comment", "<script>alert('xss')</script>")
	// 添加一个文件 part
	part, _ := writer.CreateFormFile("file", "test\r\nEvil-Header: injected.txt")
	_, _ = part.Write([]byte("file content here"))
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// comment 字段的 <script> 应被过滤
	assert.NotContains(t, body, "<script>")
	// 文件名中的 CRLF 应被清理
	assert.NotContains(t, body, "\r\nEvil-Header")
	// 文件内容应保留
	assert.Contains(t, body, "file content here")
}

// TestXSSFilter_URLEncoded_Sanitize 验证 application/x-www-form-urlencoded
// 输入的 URL 编码值被正确解码、过滤、再编码。
func TestXSSFilter_URLEncoded_Sanitize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(WithGlobalPolicy(xss.PolicyStrict)))
	r.POST("/form", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	// 使用 URL 编码，script 标签经过 QueryEscape
	form := url.Values{}
	form.Set("message", "<script>alert(1)</script>")
	form.Set("greeting", "hello world")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/form",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// <script> 应被过滤
	assert.NotContains(t, body, "<script>")
	// greeting 字段的值应保留（不含 HTML）
	parsed, err := url.ParseQuery(body)
	assert.Nil(t, err)
	assert.Equal(t, "hello world", parsed.Get("greeting"))
}

// TestXSSFilter_MaxBodySize_Exceeded 验证请求体超过大小限制时返回 400。
func TestXSSFilter_MaxBodySize_Exceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(
		WithGlobalPolicy(xss.PolicyStrict),
		WithMaxBodySize(32), // 极小限制
	))
	r.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach here")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test",
		strings.NewReader(`{"data":"this body is definitely longer than 32 bytes total"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestXSSFilter_SkipFields_PasswordNotSanitized 验证通过 WithGlobalSkipFields
// 指定的字段不被 XSS 过滤，即使包含类似 HTML 的内容也保留原值。
// 注意：passwordFieldName 只控制 trimSpace 跳过，XSS 跳过需要 WithGlobalSkipFields。
func TestXSSFilter_SkipFields_PasswordNotSanitized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(
		WithGlobalPolicy(xss.PolicyStrict),
		WithGlobalSkipFields("password"),
	))
	r.POST("/login", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/login",
		strings.NewReader(`{"password":"<script>pwd</script>","username":"<b>admin</b>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// password 在 skipField 中，应保留原始值
	assert.Contains(t, body, `<script>pwd</script>`)
	// username 非 skip 字段，<b> 应被严格策略过滤
	assert.NotContains(t, body, `<b>admin</b>`)
}

// TestXSSFilter_GlobalFieldPolicy_UGC 验证全局字段级 UGC 策略：
// 全局策略为 Strict，但 content 字段使用 UGC 策略，
// content 字段中安全的 HTML 标签（如 <b>）保留，<script> 被移除；
// 其他字段走路由 Strict 策略。
func TestXSSFilter_GlobalFieldPolicy_UGC(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(
		WithGlobalPolicy(xss.PolicyStrict),
		WithGlobalFieldPolicy(xss.PolicyUGC, "content"),
	))
	r.POST("/post", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/post",
		strings.NewReader(`{"content":"<b>hello</b><script>alert(1)</script>","title":"<i>italic</i>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// content 字段使用 UGC 策略：<b> 保留，<script> 被移除
	assert.Contains(t, body, "<b>hello</b>")
	assert.NotContains(t, body, "<script>")
	// title 字段走路由 Strict 策略，<i> 被过滤
	assert.NotContains(t, body, "<i>italic</i>")
}

// TestXSSFilter_TrimSpace 验证启用 TrimSpace 后去除值的首尾空白，
// 但密码字段不受 TrimSpace 影响。
func TestXSSFilter_TrimSpace(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(
		WithGlobalPolicy(xss.PolicyStrict),
		WithTrimSpaceEnabled(true),
	))
	r.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test",
		strings.NewReader(`{"name":"  hello  ","password":"  secret  "}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// name 字段应去除首尾空格
	assert.NotContains(t, body, `"  hello  "`)
	assert.Contains(t, body, `"hello"`)
	// password 字段不受 trimSpace 影响，保留原始值
	assert.Contains(t, body, `"  secret  "`)
}

// --- 额外覆盖：GET 请求 query string 过滤 + 多值参数 ---

func TestXSSFilter_QueryString_MultiValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(WithGlobalPolicy(xss.PolicyStrict)))
	r.GET("/search", func(c *gin.Context) {
		c.String(http.StatusOK, c.Request.URL.RawQuery)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/search?tag=<script>a</script>&tag=safe&q=<b>bold</b>", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.NotContains(t, body, "<script>")
	assert.NotContains(t, body, "<b>")
}

// --- 额外覆盖：RoutePolicy 中 fieldRule 的路由级逻辑 ---

func TestXSSFilter_RoutePolicy_FieldRule(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(
		WithGlobalPolicy(xss.PolicyStrict),
		WithRouteFieldPolicy("/admin", xss.PolicyUGC, "title", "description"),
	))
	r.POST("/admin/post", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/post",
		strings.NewReader(`{"title":"<b>hi</b><script>x</script>","description":"<b>ok</b>","name":"<b>test</b>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// title 命中路由字段 UGC 策略，<b> 保留，<script> 被过滤
	assert.Contains(t, body, `<b>hi</b>`)
	assert.NotContains(t, body, "<script>")
	// description 命中路由字段 UGC 策略，<b> 保留
	assert.Contains(t, body, `<b>ok</b>`)
	// name 走路由继承的全局 Strict 策略，<b> 被过滤
	assert.NotContains(t, body, `<b>test</b>`)
}

// --- 额外覆盖：PolicyNone 全局 + 无路由规则 → 直接跳过 ---

func TestXSSFilter_NoPolicyNoRoutes_Skip(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(WithGlobalPolicy(xss.PolicyNone)))
	r.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test",
		strings.NewReader(`{"data":"<script>alert(1)</script>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	// PolicyNone + 无路由规则 → 跳过，原始值保留
	assert.Contains(t, w.Body.String(), "<script>")
}

// --- 额外覆盖：JSON 数组输入 ---

func TestXSSFilter_JSONArray(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(WithGlobalPolicy(xss.PolicyStrict)))
	r.POST("/tags", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tags",
		strings.NewReader(`["<script>alert(1)</script>","safe"]`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "<script>")
	assert.Contains(t, w.Body.String(), "safe")
}

// --- 额外覆盖：PUT/PATCH 方法 ---

func TestXSSFilter_PutMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(WithGlobalPolicy(xss.PolicyStrict)))
	r.PUT("/item", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/item",
		strings.NewReader(`{"name":"<script>x</script>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "<script>")
}

func TestXSSFilter_PatchMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(WithGlobalPolicy(xss.PolicyStrict)))
	r.PATCH("/item", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/item",
		strings.NewReader(`{"name":"<script>x</script>"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "<script>")
}

// --- 额外覆盖：filterFormData body size exceeded ---

func TestXSSFilter_FormData_MaxBodySize_Exceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(
		WithGlobalPolicy(xss.PolicyStrict),
		WithMaxBodySize(10),
	))
	r.POST("/form", func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach here")
	})

	w := httptest.NewRecorder()
	form := url.Values{}
	form.Set("data", "this is way more than 10 bytes")
	req := httptest.NewRequest(http.MethodPost, "/form",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- 额外覆盖：multipart 中的 skipField ---

func TestXSSFilter_MultipartForm_SkipField(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(
		WithGlobalPolicy(xss.PolicyStrict),
		WithGlobalSkipFields("raw_input"),
	))
	r.POST("/upload", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("raw_input", "<script>alert(1)</script>")
	_ = writer.WriteField("comment", "<b>hello</b>")
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// raw_input 是 skipField，保留原始值
	assert.Contains(t, body, "<script>alert(1)</script>")
	// comment 非 skipField，<b> 被严格策略过滤
	assert.NotContains(t, body, "<b>hello</b>")
}

// --- 额外覆盖：filterJSON nil body / http.NoBody ---

func TestXSSFilter_JSON_NilBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &handler{
		xssRuleItem: xssRuleItem{
			policy:     nil,
			skipField:  make(map[string]struct{}),
			fieldRules: make(map[string]*bluemonday.Policy),
		},
		passwordFieldName: []string{"password"},
		routePolicies:     make(map[string]*xssRuleItem),
		skipRoutes:        make(map[string]struct{}),
	}
	h.policy = h.makePolicy(xss.PolicyStrict)

	c, _ := newTestContext(http.MethodPost, "/test", nil, "application/json")
	err := h.filterJSON(c)
	assert.Nil(t, err)
}

func TestXSSFilter_JSON_NoBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &handler{
		xssRuleItem: xssRuleItem{
			policy:     nil,
			skipField:  make(map[string]struct{}),
			fieldRules: make(map[string]*bluemonday.Policy),
		},
		passwordFieldName: []string{"password"},
		routePolicies:     make(map[string]*xssRuleItem),
		skipRoutes:        make(map[string]struct{}),
	}
	h.policy = h.makePolicy(xss.PolicyStrict)

	c, _ := newTestContext(http.MethodPost, "/test", http.NoBody, "application/json")
	err := h.filterJSON(c)
	assert.Nil(t, err)
}

// --- 额外覆盖：filterFormData nil body ---

func TestXSSFilter_FormData_NilBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	h := &handler{
		xssRuleItem: xssRuleItem{
			policy:     nil,
			skipField:  make(map[string]struct{}),
			fieldRules: make(map[string]*bluemonday.Policy),
		},
		passwordFieldName: []string{"password"},
		routePolicies:     make(map[string]*xssRuleItem),
		skipRoutes:        make(map[string]struct{}),
	}
	h.policy = h.makePolicy(xss.PolicyStrict)

	c, _ := newTestContext(http.MethodPost, "/test", nil, "application/x-www-form-urlencoded")
	err := h.filterFormData(c)
	assert.Nil(t, err)
}

// --- 额外覆盖：multipart 中包含文件 part 的 Content-Type 头保留 ---

func TestXSSFilter_MultipartForm_FileWithContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(WithGlobalPolicy(xss.PolicyStrict)))
	r.POST("/upload", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		c.String(http.StatusOK, string(body))
	})

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, _ := writer.CreateFormFile("file", "report.pdf")
	_, _ = part.Write([]byte("PDF content"))
	_ = writer.WriteField("title", "<script>x</script>")
	writer.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	// 文件 part 应包含 Content-Type
	assert.Contains(t, body, "Content-Type:")
	// title 字段应被过滤
	assert.NotContains(t, body, "<script>")
	// 文件内容应保留
	assert.Contains(t, body, "PDF content")
}

// --- 额外覆盖：filterJSON 嵌套对象 ---

func TestXSSFilter_JSON_NestedObject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(WithGlobalPolicy(xss.PolicyStrict)))
	r.POST("/test", func(c *gin.Context) {
		body, _ := c.GetRawData()
		c.String(http.StatusOK, string(body))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test",
		strings.NewReader(`{"user":{"name":"<script>x</script>","age":25}}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "<script>")
}

// --- 额外覆盖：filterJSON invalid JSON returns error ---

func TestXSSFilter_JSON_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(WithGlobalPolicy(xss.PolicyStrict)))
	r.POST("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "should not reach here")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/test",
		strings.NewReader(`{invalid json}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- 额外覆盖：TrimSpace 对 query string 和 form data ---

func TestXSSFilter_TrimSpace_QueryString(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(New(
		WithGlobalPolicy(xss.PolicyStrict),
		WithTrimSpaceEnabled(true),
	))
	r.GET("/search", func(c *gin.Context) {
		c.String(http.StatusOK, c.Request.URL.RawQuery)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=++hello++", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	parsed, err := url.ParseQuery(body)
	assert.Nil(t, err)
	assert.Equal(t, "hello", parsed.Get("q"))
}
