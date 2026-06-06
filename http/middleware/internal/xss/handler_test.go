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

func init() {
	gin.SetMode(gin.TestMode)
}

// --- matchRoute tests ---

func TestMatchRoute_ExactMatch(t *testing.T) {
	assert.True(t, matchRoute("/user", "/user"))
	assert.True(t, matchRoute("/api/v1/login", "/api/v1/login"))
	assert.True(t, matchRoute("/", "/"))
}

func TestMatchRoute_PrefixMatch(t *testing.T) {
	assert.True(t, matchRoute("/user/list", "/user"))
	assert.True(t, matchRoute("/api/v1/data", "/api/v1"))
}

func TestMatchRoute_NoPartialMatch(t *testing.T) {
	// /user should NOT match /user-admin
	assert.False(t, matchRoute("/user-admin", "/user"))
	// /api should NOT match /api2
	assert.False(t, matchRoute("/api2", "/api"))
	// /a should NOT match /ab
	assert.False(t, matchRoute("/ab", "/a"))
}

func TestMatchRoute_EdgeCases(t *testing.T) {
	// exact match on empty strings
	assert.True(t, matchRoute("", ""))
	// empty requestPath with non-empty route
	assert.False(t, matchRoute("", "/user"))
}

func TestMatchRoute_TrailingSlash(t *testing.T) {
	assert.True(t, matchRoute("/user/", "/user"))
	assert.True(t, matchRoute("/user/", "/user/"))
}

// newTestContext creates a gin.Context with a FullPath set via a real gin router.
func newTestContext(method, path string, body io.Reader, contentType string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	r := gin.New()

	var captured *gin.Context
	r.Handle(method, path, func(ctx *gin.Context) {
		captured = ctx
		ctx.Next()
	})

	req := httptest.NewRequest(method, path, body)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	r.ServeHTTP(w, req)
	return captured, w
}

// --- filterFormData multi-value tests ---

func TestFilterFormData_MultiValueField(t *testing.T) {
	h := &handler{
		xssRuleItem: xssRuleItem{
			policy:    nil,
			skipField: make(map[string]struct{}),
		},
		passwordFieldName: []string{"password"},
		routePolicies:     make(map[string]*xssRuleItem),
		skipRoutes:        make(map[string]struct{}),
	}
	h.policy = h.makePolicy(xss.PolicyStrict)

	c, _ := newTestContext(http.MethodPost, "/test", strings.NewReader("tags=go&tags=rust&name=hello"), "application/x-www-form-urlencoded")

	err := h.filterFormData(c)
	assert.Nil(t, err)

	body, _ := io.ReadAll(c.Request.Body)
	result := string(body)

	// Both "tags" values should be present
	parsed, err := url.ParseQuery(result)
	assert.Nil(t, err)
	assert.Equal(t, []string{"go", "rust"}, parsed["tags"])
	assert.Equal(t, []string{"hello"}, parsed["name"])
}

func TestFilterFormData_SingleValueField(t *testing.T) {
	h := &handler{
		xssRuleItem: xssRuleItem{
			policy:    nil,
			skipField: make(map[string]struct{}),
		},
		passwordFieldName: []string{"password"},
		routePolicies:     make(map[string]*xssRuleItem),
		skipRoutes:        make(map[string]struct{}),
	}
	h.policy = h.makePolicy(xss.PolicyStrict)

	c, _ := newTestContext(http.MethodPost, "/test", strings.NewReader("name=hello"), "application/x-www-form-urlencoded")

	err := h.filterFormData(c)
	assert.Nil(t, err)

	body, _ := io.ReadAll(c.Request.Body)
	parsed, err := url.ParseQuery(string(body))
	assert.Nil(t, err)
	assert.Equal(t, []string{"hello"}, parsed["name"])
}

// --- multipart boundary parsing tests ---

func TestFilterMultipartFormData_BoundaryParsing(t *testing.T) {
	h := &handler{
		xssRuleItem: xssRuleItem{
			policy:    nil,
			skipField: make(map[string]struct{}),
		},
		passwordFieldName: []string{"password"},
		routePolicies:     make(map[string]*xssRuleItem),
		skipRoutes:        make(map[string]struct{}),
	}
	h.policy = h.makePolicy(xss.PolicyStrict)

	// Build a multipart body
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	boundary := writer.Boundary()

	_ = writer.WriteField("username", "hello")
	_ = writer.WriteField("bio", "<script>alert(1)</script>")
	writer.Close()

	// Content-Type with extra parameters after boundary
	ct := "multipart/form-data; boundary=" + boundary + "; charset=utf-8"

	c, _ := newTestContext(http.MethodPost, "/upload", &buf, ct)

	err := h.filterMultiPartFormData(c)
	assert.Nil(t, err)

	body, _ := io.ReadAll(c.Request.Body)
	bodyStr := string(body)

	// The boundary should be correctly extracted and the body properly parsed
	assert.Contains(t, bodyStr, boundary)
	assert.Contains(t, bodyStr, "username")
	assert.Contains(t, bodyStr, "hello")
	// The <script> tag should be stripped by strict policy
	assert.NotContains(t, bodyStr, "<script>")
}

func TestFilterMultipartFormData_MissingBoundary(t *testing.T) {
	h := &handler{
		xssRuleItem: xssRuleItem{
			policy:    nil,
			skipField: make(map[string]struct{}),
		},
		passwordFieldName: []string{"password"},
		routePolicies:     make(map[string]*xssRuleItem),
		skipRoutes:        make(map[string]struct{}),
	}

	c, _ := newTestContext(http.MethodPost, "/upload", strings.NewReader(""), "multipart/form-data")

	err := h.filterMultiPartFormData(c)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "boundary")
}

func TestFilterMultipartFormData_InvalidContentType(t *testing.T) {
	h := &handler{
		xssRuleItem: xssRuleItem{
			policy:    nil,
			skipField: make(map[string]struct{}),
		},
		passwordFieldName: []string{"password"},
		routePolicies:     make(map[string]*xssRuleItem),
		skipRoutes:        make(map[string]struct{}),
	}

	c, _ := newTestContext(http.MethodPost, "/upload", strings.NewReader(""), "invalid-content-type")

	err := h.filterMultiPartFormData(c)
	assert.NotNil(t, err)
}

// --- route matching integration with filterXSS ---

func TestFilterXSS_RouteMatching(t *testing.T) {
	h := &handler{
		xssRuleItem: xssRuleItem{
			policy:     nil,
			skipField:  make(map[string]struct{}),
			fieldRules: make(map[string]*bluemonday.Policy),
		},
		passwordFieldName: []string{"password"},
		routePolicies: map[string]*xssRuleItem{
			"/user": {
				policy:    bluemonday.StrictPolicy(),
				skipField: make(map[string]struct{}),
			},
		},
		skipRoutes: make(map[string]struct{}),
	}
	h.policy = h.makePolicy(xss.PolicyStrict)

	// /user should match /user route and sanitize via route policy
	result := h.filterXSS("/user", "name", "<b>hello</b>")
	assert.NotContains(t, result, "<b>")

	// /user-admin should NOT match /user route, falls through to global policy
	result2 := h.filterXSS("/user-admin", "name", "<b>hello</b>")
	assert.NotContains(t, result2, "<b>")
}

func TestSanitizeFilename(t *testing.T) {
	t.Run("removes CRLF", func(t *testing.T) {
		result := sanitizeFilename("hello\r\nX-Injected: true.txt")
		assert.Equal(t, "helloX-Injected: true.txt", result)
	})
	t.Run("removes null bytes", func(t *testing.T) {
		result := sanitizeFilename("file\x00name.txt")
		assert.Equal(t, "filename.txt", result)
	})
	t.Run("clean filename unchanged", func(t *testing.T) {
		result := sanitizeFilename("report.pdf")
		assert.Equal(t, "report.pdf", result)
	})
}
