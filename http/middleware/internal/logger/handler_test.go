package logger

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/xerror"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newLoggerTestContext 创建带请求的 gin.Context
func newLoggerTestContext(method, path string, body string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	if method == http.MethodPost {
		c.Request.Header.Set("Content-Type", "application/json")
	}
	return c
}

// TestNew_WithRedact 启用脱敏模式
func TestNew_WithRedact(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	l := New(c, true, nil)

	assert.NotNil(t, l)
	assert.True(t, c.Writer.(*responseWriter) != nil)
}

// TestNew_WithoutRedact 禁用脱敏模式
func TestNew_WithoutRedact(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	l := New(c, false, nil)

	assert.NotNil(t, l)
	assert.False(t, l.(*handler).redactEnabled)
}

// TestNew_CustomSensitiveFields 自定义敏感字段
func TestNew_CustomSensitiveFields(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	l := New(c, true, []string{"apiKey", "secret"})

	h := l.(*handler)
	assert.True(t, h.sensitiveKeys["apikey"])
	assert.True(t, h.sensitiveKeys["secret"])
}

// TestNew_DefaultSensitiveFields 启用脱敏但未指定字段时使用默认列表
func TestNew_DefaultSensitiveFields(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	l := New(c, true, nil)

	h := l.(*handler)
	assert.True(t, h.sensitiveKeys["password"])
	assert.True(t, h.sensitiveKeys["token"])
	assert.True(t, h.sensitiveKeys["secret"])
}

// TestHandler_General_GET GET 请求显示方法
func TestHandler_General_GET(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/api/users", "")
	l := New(c, false, nil)

	output := l.String()
	assert.Contains(t, output, "[GET]")
	assert.Contains(t, output, "/api/users")
}

// TestHandler_General_POST POST 请求显示方法
func TestHandler_General_POST(t *testing.T) {
	c := newLoggerTestContext(http.MethodPost, "/api/users", `{"name":"test"}`)
	l := New(c, false, nil)

	output := l.String()
	assert.Contains(t, output, "[POST]")
}

// TestHandler_Request_Headers 显示请求头
func TestHandler_Request_Headers(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	c.Request.Header.Set("X-Custom", "value")
	l := New(c, false, nil)

	output := l.String()
	assert.Contains(t, output, "[Request]")
	assert.Contains(t, output, "[HEADER]")
	assert.Contains(t, output, "X-Custom")
	assert.Contains(t, output, "value")
}

// TestHandler_Request_RedactSensitiveHeaders 脱敏敏感请求头
func TestHandler_Request_RedactSensitiveHeaders(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	c.Request.Header.Set("Authorization", "Bearer very-long-secret-token")
	c.Request.Header.Set("Cookie", "session=abc123")
	l := New(c, true, nil)

	output := l.String()
	// 敏感头应被脱敏
	assert.NotContains(t, output, "very-long-secret-token")
	assert.NotContains(t, output, "session=abc123")
	assert.Contains(t, output, "Bear****")
}

// TestHandler_Request_RedactShortValue 短值完全替换为 ****
func TestHandler_Request_RedactShortValue(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	c.Request.Header.Set("Authorization", "abc")
	l := New(c, true, nil)

	output := l.String()
	assert.Contains(t, output, "****")
	assert.NotContains(t, output, "abc")
}

// TestHandler_Request_GETNoPayload GET 请求无 payload 不显示
func TestHandler_Request_GETNoPayload(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	l := New(c, false, nil)

	output := l.String()
	assert.NotContains(t, output, "[PAYLOAD]")
}

// TestHandler_Request_POSTWithPayload POST 请求显示 payload
func TestHandler_Request_POSTWithPayload(t *testing.T) {
	c := newLoggerTestContext(http.MethodPost, "/test", `{"name":"test"}`)
	l := New(c, false, nil)

	output := l.String()
	assert.Contains(t, output, "[PAYLOAD]")
	assert.Contains(t, output, `"name"`)
}

// TestHandler_Request_POSTNilPayload POST 请求空 payload 显示 <nil>
func TestHandler_Request_POSTNilPayload(t *testing.T) {
	c := newLoggerTestContext(http.MethodPost, "/test", "")
	l := New(c, false, nil)

	output := l.String()
	assert.Contains(t, output, "<nil>")
}

// TestHandler_Request_RedactJSONPayload JSON payload 脱敏
func TestHandler_Request_RedactJSONPayload(t *testing.T) {
	c := newLoggerTestContext(http.MethodPost, "/test", `{"username":"john","password":"secret123"}`)
	c.Request.Header.Set("Content-Type", "application/json")
	l := New(c, true, nil)

	output := l.String()
	assert.NotContains(t, output, "secret123")
	assert.Contains(t, output, "***")
}

// TestHandler_Request_RedactFormPayload form payload 脱敏
func TestHandler_Request_RedactFormPayload(t *testing.T) {
	c := newLoggerTestContext(http.MethodPost, "/test", "username=john&password=secret123")
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	l := New(c, true, nil)

	output := l.String()
	assert.NotContains(t, output, "secret123")
}

// TestHandler_Request_NoRedactPayload 禁用脱敏时 payload 原样显示
func TestHandler_Request_NoRedactPayload(t *testing.T) {
	c := newLoggerTestContext(http.MethodPost, "/test", `{"username":"john","password":"secret123"}`)
	c.Request.Header.Set("Content-Type", "application/json")
	l := New(c, false, nil)

	output := l.String()
	assert.Contains(t, output, "secret123")
}

// TestHandler_Request_FileUpload 文件上传时显示占位符
func TestHandler_Request_FileUpload(t *testing.T) {
	body := "--boundary123\r\nContent-Disposition: form-data; name=\"file\"; filename=\"test.txt\"\r\n\r\nfile content here\r\n--boundary123--"
	c := newLoggerTestContext(http.MethodPost, "/test", body)
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=boundary123")
	l := New(c, false, nil)

	output := l.String()
	assert.Contains(t, output, ">>>> FILE DATA <<<<")
}

// TestHandler_Response 显示响应状态和头
func TestHandler_Response(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	l := New(c, false, nil)

	// 写入响应
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Header().Set("X-Response", "value")

	output := l.String()
	assert.Contains(t, output, "[Response]")
	assert.Contains(t, output, "[STATUS]")
	assert.Contains(t, output, "[BODY]")
}

// TestHandler_Response_NilBody 响应体为空显示 <nil>
func TestHandler_Response_NilBody(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	l := New(c, false, nil)

	// 不写入任何响应体
	output := l.String()
	assert.Contains(t, output, "<nil>")
}

// TestHandler_Error_GinErrors 显示 gin 错误
func TestHandler_Error_GinErrors(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	l := New(c, false, nil)

	// 添加 gin 错误
	_ = c.Error(errors.New("something went wrong"))

	output := l.String()
	assert.Contains(t, output, "[Error]")
	assert.Contains(t, output, "something went wrong")
}

// TestHandler_Error_NoErrors 无错误时不显示
func TestHandler_Error_NoErrors(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	l := New(c, false, nil)

	output := l.String()
	assert.NotContains(t, output, "[Error]")
}

// TestHandler_Error_XErrorWithFields xerror 带 fields 信息
func TestHandler_Error_XErrorWithFields(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	l := New(c, false, nil)

	// 添加带 fields 的 xerror
	xe := xerror.New("db error").WithFields(xerror.F("query", "SELECT *"), xerror.F("table", "users"))
	_ = c.Error(xe)

	output := l.String()
	assert.Contains(t, output, "[Error]")
	assert.Contains(t, output, "[FIELDS]")
}

// TestRedactJSON_InvalidJSON 无效 JSON 原样返回
func TestRedactJSON_InvalidJSON(t *testing.T) {
	h := &handler{
		redactEnabled: true,
		sensitiveKeys: map[string]bool{"password": true},
	}

	result := h.redactJSON([]byte(`not valid json`))
	assert.Equal(t, []byte("not valid json"), result)
}

// TestRedactJSON_NestedJSON 嵌套 JSON 脱敏
func TestRedactJSON_NestedJSON(t *testing.T) {
	h := &handler{
		redactEnabled: true,
		sensitiveKeys: map[string]bool{"password": true},
	}

	input := []byte(`{"user":{"password":"secret"},"name":"john"}`)
	result := h.redactJSON(input)
	assert.NotContains(t, string(result), "secret")
	assert.Contains(t, string(result), "***")
}

// TestRedactJSON_ArrayJSON 顶层数组不被脱敏（redactJSONValue 只处理 map）
func TestRedactJSON_ArrayJSON(t *testing.T) {
	h := &handler{
		redactEnabled: true,
		sensitiveKeys: map[string]bool{"password": true},
	}

	// 顶层数组：redactJSONValue 不处理顶层数组，原样返回
	input := []byte(`[{"password":"secret1"},{"password":"secret2"}]`)
	result := h.redactJSON(input)
	// 顶层数组不被脱敏（代码限制）
	assert.Contains(t, string(result), "secret1")
}

// TestRedactFormData_InvalidForm 无效 form data 原样返回
func TestRedactFormData_InvalidForm(t *testing.T) {
	h := &handler{
		redactEnabled: true,
		sensitiveKeys: map[string]bool{"password": true},
	}

	// 无效的 form data（包含 %zz 无法解析）
	result := h.redactFormData([]byte("name=%zz"))
	assert.Equal(t, []byte("name=%zz"), result)
}

// TestRedactBody_NotJSONOrForm 非 JSON/Form 内容类型不脱敏
func TestRedactBody_NotJSONOrForm(t *testing.T) {
	h := &handler{
		redactEnabled: true,
		sensitiveKeys: map[string]bool{"password": true},
	}

	body := []byte("some plain text with password in it")
	result := h.redactBody(body, "text/plain")
	assert.Equal(t, body, result)
}

// TestPrintError_NilError nil 错误返回空字符串
func TestPrintError_NilError(t *testing.T) {
	h := &handler{}
	result := h.printError(nil)
	assert.Empty(t, result)
}

// TestRedactValue_ShortValue 短值替换为 ****
func TestRedactValue_ShortValue(t *testing.T) {
	h := &handler{redactEnabled: true}
	assert.Equal(t, "****", h.redactValue("ab"))
	assert.Equal(t, "****", h.redactValue(""))
}

// TestRedactValue_LongValue 长值保留前4字符
func TestRedactValue_LongValue(t *testing.T) {
	h := &handler{redactEnabled: true}
	assert.Equal(t, "Bear****", h.redactValue("Bearer token"))
}

// TestResponseWriter_Write 自定义 ResponseWriter 的 Write 方法
func TestResponseWriter_Write(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = &http.Request{Header: make(http.Header)}
	rw := newResponseWriter(c.Writer)

	n, err := rw.Write([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", rw.body.String())
}

// TestILogger_Interface 编译时接口检查
func TestILogger_Interface(t *testing.T) {
	var _ ILogger = (*handler)(nil)
}

// TestNew_SetsWriter 替换 gin.Context 的 Writer
func TestNew_SetsWriter(t *testing.T) {
	c := newLoggerTestContext(http.MethodGet, "/test", "")
	originalWriter := c.Writer

	New(c, false, nil)

	assert.NotEqual(t, originalWriter, c.Writer, "New should replace c.Writer with responseWriter")
	_, ok := c.Writer.(*responseWriter)
	assert.True(t, ok, "c.Writer should be a responseWriter")
}

// TestHandler_String_CompleteOutput 完整输出包含所有部分
func TestHandler_String_CompleteOutput(t *testing.T) {
	c := newLoggerTestContext(http.MethodPost, "/api/login", `{"username":"john","password":"pass123"}`)
	c.Request.Header.Set("Content-Type", "application/json")
	l := New(c, true, nil)

	// 写入响应
	c.Writer.WriteHeader(http.StatusOK)

	output := l.String()
	assert.Contains(t, output, "[POST]")
	assert.Contains(t, output, "/api/login")
	assert.Contains(t, output, "[Request]")
	assert.Contains(t, output, "[Response]")
}
