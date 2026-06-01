package restful

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestContext() (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, "/test?start=0&limit=20", nil)
	return w, c
}

func newTestContextWithURI(uri string) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(http.MethodGet, uri, nil)
	return w, c
}

// --- Retrieve ---

func TestRetrieve_Success(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	entity := map[string]string{"id": "1", "name": "test"}
	r.Retrieve(entity)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "1", body["id"])
	assert.Equal(t, "test", body["name"])
}

func TestRetrieve_NotFound(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.Retrieve(nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Post ---

func TestPost_Success(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	entity := map[string]string{"id": "2", "name": "created"}
	r.Post(entity)

	assert.Equal(t, http.StatusCreated, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "2", body["id"])
}

func TestPost_NilEntity(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.Post(nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Put ---

func TestPut_Success(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	entity := map[string]string{"id": "1", "name": "updated"}
	r.Put(entity)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "updated", body["name"])
}

func TestPut_NilEntity(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.Put(nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- Patch ---

func TestPatch_WithEntity(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	entity := map[string]string{"id": "1", "name": "patched"}
	r.Patch(entity)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "patched", body["name"])
}

func TestPatch_NilEntity_NoContent(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.Patch(nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

// --- Delete ---

func TestDelete_Success(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.Delete(nil)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestDelete_WithError(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c, WithResponseDebugError(true))

	r.Delete(fmt.Errorf("delete failed"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "delete failed", body["message"])
}

// --- WithError ---

func TestWithError_XError(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	xerr := xerror.NewXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	)
	r.WithError(xerr)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "bad request", body["message"])

	// Check error code header
	assert.Equal(t, "10001", w.Header().Get(ErrorCodeHeaderKey))
}

func TestWithError_StandardError(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.WithError(errors.New("standard error"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, http.StatusText(http.StatusInternalServerError), body["message"])
}

func TestWithError_NilError(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.WithError(nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "error not defined", body["message"])
}

// --- WithMessage ---

func TestWithMessage(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.WithMessage("operation successful")

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "operation successful", body["message"])
}

func TestWithMessage_Empty(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.WithMessage("")

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "success", body["message"])
}

// --- SetHeader ---

func TestSetHeader_XPrefix(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	result := r.SetHeader("X-Custom-Header", "test-value")
	assert.NotNil(t, result) // Should return IResponse for chaining

	// Trigger a response to flush headers
	r.WithMessage("ok")

	assert.Equal(t, "test-value", w.Header().Get("X-Custom-Header"))
}

func TestSetHeader_LowercaseXPrefix(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.SetHeader("x-another-header", "another-value")
	r.WithMessage("ok")

	assert.Equal(t, "another-value", w.Header().Get("x-Another-Header"))
}

func TestSetHeader_NonXPrefix_StandardWhitelist(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	// Content-Type 在标准头白名单中，严格模式下也应被允许
	r.SetHeader("Content-Type", "application/json")
	r.WithMessage("ok")

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestSetHeader_NonXPrefix_NonWhitelisted_Ignored(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	// 非白名单且非 X- 前缀的 header 应被忽略
	r.SetHeader("X-Secret", "secret-value") // X- 前缀应被设置
	r.SetHeader("Some-Random-Header", "random")
	r.WithMessage("ok")

	assert.Equal(t, "secret-value", w.Header().Get("X-Secret"))
	assert.Equal(t, "", w.Header().Get("Some-Random-Header"))
}

func TestSetHeader_RelaxedHeaders_AllowsNonXPrefix(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c, WithResponseRelaxedHeaders())

	r.SetHeader("Content-Type", "application/json")
	r.WithMessage("ok")

	// 放松模式下，非 X- 前缀的请求头应被设置
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestSetHeader_RelaxedHeaders_StillAllowsXPrefix(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c, WithResponseRelaxedHeaders())

	r.SetHeader("X-Custom-Header", "test-value")
	r.WithMessage("ok")

	// 放松模式下，X- 前缀的请求头仍然正常工作
	assert.Equal(t, "test-value", w.Header().Get("X-Custom-Header"))
}

func TestSetHeader_DefaultStrictMode(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	// Cache-Control 在标准头白名单中，严格模式下也应被允许
	r.SetHeader("Cache-Control", "no-cache")
	r.WithMessage("ok")

	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
}

// --- TableWithPagination ---

func TestTableWithPagination(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	resp := &TableResponse{
		TotalRow: 2,
		Columns:  []string{"name", "age"},
		RowKeys:  []string{"row1", "row2"},
		Items: []*TableResponseItem{
			{Column: "name", RowKey: "row1", Data: "Alice"},
			{Column: "age", RowKey: "row1", Data: 30},
			{Column: "name", RowKey: "row2", Data: "Bob"},
			{Column: "age", RowKey: "row2", Data: 25},
		},
		Extends: []*TableResponseRowExtendItem{
			{RowKey: "row1", Data: "extra1"},
		},
	}
	r.TableWithPagination(resp)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)

	// Check structure
	assert.Contains(t, body, "columns")
	assert.Contains(t, body, "rowKeys")
	assert.Contains(t, body, "data")
	assert.Contains(t, body, "extends")

	// Check total count header
	assert.Equal(t, "2", w.Header().Get(TotalCountHeaderKey))

	// Check pagination info header exists
	assert.NotEmpty(t, w.Header().Get(PageInfoHeaderKey))

	// Check link header exists
	assert.NotEmpty(t, w.Header().Get(PageLinkHeaderKey))
}

// --- ListWithPagination ---

func TestListWithPagination(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	items := []map[string]string{
		{"id": "1", "name": "Alice"},
		{"id": "2", "name": "Bob"},
	}
	r.ListWithPagination(2, items)

	assert.Equal(t, http.StatusOK, w.Code)

	var body []map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Len(t, body, 2)
	assert.Equal(t, "Alice", body[0]["name"])

	// Check pagination headers
	assert.Equal(t, "2", w.Header().Get(TotalCountHeaderKey))
	assert.NotEmpty(t, w.Header().Get(PageInfoHeaderKey))
}

func TestListWithPagination_EmptySlice(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	var items []map[string]string
	r.ListWithPagination(0, items)

	assert.Equal(t, http.StatusOK, w.Code)

	var body []any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Empty(t, body)
}

func TestListWithPagination_NonSliceValue(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	// Non-slice values are now passed through without type checking
	r.ListWithPagination(0, "not-a-slice")

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestListWithPagination_NilSliceReturnsEmptyArray(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	var users []map[string]string // nil slice
	r.ListWithPagination(0, users)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `[]`, w.Body.String())
}

// --- ListWithMoreFlag ---

func TestListWithMoreFlag(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	items := []map[string]string{{"id": "1"}}
	r.ListWithMoreFlag(true, items)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "true", w.Header().Get(HasMoreHeaderKey))
}

func TestListWithMoreFlag_NilSliceReturnsEmptyArray(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	var users []map[string]string // nil slice
	r.ListWithMoreFlag(false, users)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `[]`, w.Body.String())
}

// --- WithErrorData ---

func TestWithErrorData(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	errData := map[string]string{"field": "email", "reason": "invalid"}
	r.WithErrorData(fmt.Errorf("validation failed"), errData)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	// Check that error data is in header
	errorDataHeader := w.Header().Get(ErrorDataHeaderKey)
	assert.NotEmpty(t, errorDataHeader)

	var parsed map[string]string
	err := json.Unmarshal([]byte(errorDataHeader), &parsed)
	assert.Nil(t, err)
	assert.Equal(t, "email", parsed["field"])
	assert.Equal(t, "invalid", parsed["reason"])
}

// --- WithBody ---

func TestWithBody(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	r.WithBody("plain text body")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "plain text body", w.Body.String())
}

func TestResponse_WithError_NonXError_HidesInternalDetail(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	resp := NewResponse(c)
	resp.WithError(errors.New("sql: connection refused at 10.0.0.1:5432"))

	// 默认模式：非 xerror 错误不应暴露内部细节
	assert.NotContains(t, w.Body.String(), "sql:")
	assert.NotContains(t, w.Body.String(), "10.0.0.1")
	assert.Contains(t, w.Body.String(), "Internal Server Error")
}

func TestResponse_WithError_NonXError_DebugModeShowsDetail(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	resp := NewResponse(c, WithResponseDebugError(true))
	resp.WithError(errors.New("sql: connection refused at 10.0.0.1:5432"))

	// 调试模式：显示原始错误
	assert.Contains(t, w.Body.String(), "sql: connection refused")
}

// --- WithResponseShowXCode (错误码白名单) ---

func TestWithResponseShowXCode_EmptyShowsAll(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	xerr := xerror.NewXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	)
	r.WithError(xerr)

	// 默认空白名单 = 全部可见
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "bad request", body["message"])
}

func TestWithResponseShowXCode_OnlyListedVisible(t *testing.T) {
	w, c := newTestContext()
	// 仅允许 10001 错误码可见
	r := NewResponse(c, WithResponseShowXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	))

	// 10001 应可见
	xerr := xerror.NewXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	)
	r.WithError(xerr)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "bad request", body["message"])

	// 99999 不在白名单，应展示通用消息
	w2, c2 := newTestContext()
	r2 := NewResponse(c2, WithResponseShowXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	))

	xerr2 := xerror.NewXCode(
		xcode.NewWithHTTPStatus(99999, http.StatusBadRequest, "sensitive info"),
	)
	r2.WithError(xerr2)
	assert.Equal(t, http.StatusBadRequest, w2.Code)

	var body2 map[string]string
	err = json.Unmarshal(w2.Body.Bytes(), &body2)
	assert.Nil(t, err)
	assert.NotContains(t, body2["message"], "sensitive info")
	assert.Equal(t, xcode.InternalServerError.String(), body2["message"])
}

// --- ListWithCursor ---

func TestListWithCursor_WithNextCursor(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	items := []map[string]string{{"id": "1"}}
	r.ListWithCursor("cursor_abc123", items)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "cursor_abc123", w.Header().Get(NextCursorHeaderKey))

	var body []map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Len(t, body, 1)
}

func TestListWithCursor_EmptyCursor(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	items := []map[string]string{{"id": "1"}}
	r.ListWithCursor("", items)

	assert.Equal(t, http.StatusOK, w.Code)
	// Empty cursor should not set X-Next-Cursor header
	assert.Equal(t, "", w.Header().Get(NextCursorHeaderKey))
}

func TestListWithCursor_NilSlice(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	var items []map[string]string
	r.ListWithCursor("next_page", items)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "next_page", w.Header().Get(NextCursorHeaderKey))
	assert.JSONEq(t, `[]`, w.Body.String())
}

// --- WithResponseLanguageHeaderKey ---

func TestWithResponseLanguageHeaderKey(t *testing.T) {
	w, c := newTestContext()
	c.Request.Header.Set("X-Language", "en-US")

	r := NewResponse(c,
		WithResponseErrorMsgHandler(
			[]language.Tag{language.AmericanEnglish, language.SimplifiedChinese},
			func(code int, lang language.Tag) string {
				if lang == language.AmericanEnglish {
					return "English error"
				}
				return ""
			},
		),
	)

	xerr := xerror.NewXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	)
	r.WithError(xerr)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "English error", body["message"])
}

// --- WithResponseLanguageHeaderKey with custom header key ---

func TestWithResponseLanguageHeaderKey_CustomKey(t *testing.T) {
	w, c := newTestContext()
	c.Request.Header.Set("X-Custom-Lang", "en-US")

	r := NewResponse(c,
		WithResponseErrorMsgHandler(
			[]language.Tag{language.AmericanEnglish},
			func(code int, lang language.Tag) string {
				if lang == language.AmericanEnglish {
					return "custom header error"
				}
				return ""
			},
		),
		WithResponseLanguageHeaderKey("X-Custom-Lang"), // override the key set by WithResponseErrorMsgHandler
	)

	xerr := xerror.NewXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	)
	r.WithError(xerr)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "custom header error", body["message"])
}

// --- WithResponseDefaultLanguage ---

func TestWithResponseDefaultLanguage(t *testing.T) {
	w, c := newTestContext()
	// No Accept-Language header set

	r := NewResponse(c,
		WithResponseDefaultLanguage(language.AmericanEnglish),
		WithResponseErrorMsgHandler(
			[]language.Tag{language.AmericanEnglish},
			func(code int, lang language.Tag) string {
				if lang == language.AmericanEnglish {
					return "English error message"
				}
				return ""
			},
		),
	)

	xerr := xerror.NewXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	)
	r.WithError(xerr)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "English error message", body["message"])
}

// --- WithResponseLogger ---

func TestWithResponseLogger(t *testing.T) {
	w, c := newTestContext()
	logger := slog.Default()
	r := NewResponse(c, WithResponseLogger(logger))

	r.WithMessage("with custom logger")
	assert.Equal(t, http.StatusOK, w.Code)
}

// --- WithResponseShowXCode empty call resets whitelist ---

func TestWithResponseShowXCode_EmptyCallResetsWhitelist(t *testing.T) {
	w, c := newTestContext()
	// Call with no xcodes → empty whitelist = all visible
	r := NewResponse(c, WithResponseShowXCode())

	xerr := xerror.NewXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	)
	r.WithError(xerr)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "bad request", body["message"])
}

// --- WithErrorData with marshal error ---

func TestWithErrorData_MarshalError(t *testing.T) {
	w, c := newTestContext()
	r := NewResponse(c)

	// Pass a value that cannot be marshaled (channel is not JSON-serializable)
	r.WithErrorData(errors.New("test"), make(chan int))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Contains(t, body["message"], "marshal failed")
}

// --- detectLanguage: X-Language header ---

func TestDetectLanguage_XLanguageHeader(t *testing.T) {
	w, c := newTestContext()
	c.Request.Header.Set("X-Language", "en-US")

	r := NewResponse(c,
		WithResponseErrorMsgHandler(
			[]language.Tag{language.AmericanEnglish},
			func(code int, lang language.Tag) string {
				if lang == language.AmericanEnglish {
					return "English message"
				}
				return ""
			},
		),
	)

	xerr := xerror.NewXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	)
	r.WithError(xerr)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "English message", body["message"])
}

// --- detectLanguage: invalid Accept-Language returns default language ---

func TestDetectLanguage_InvalidAcceptLanguage(t *testing.T) {
	w, c := newTestContext()
	c.Request.Header.Set("Accept-Language", "!!invalid!!")

	r := NewResponse(c,
		WithResponseDefaultLanguage(language.AmericanEnglish),
		WithResponseErrorMsgHandler(
			[]language.Tag{language.AmericanEnglish},
			func(code int, lang language.Tag) string {
				return "fallback"
			},
		),
	)

	xerr := xerror.NewXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "bad request"),
	)
	r.WithError(xerr)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	// Invalid Accept-Language should fall back to default language
	assert.Equal(t, "fallback", body["message"])
}

// --- detectLanguage: no supported languages returns default ---

func TestDetectLanguage_NoSupportedLanguages(t *testing.T) {
	w, c := newTestContext()

	r := NewResponse(c,
		WithResponseDefaultLanguage(language.AmericanEnglish),
	)

	// With no supported languages and no msgHandler, detectLanguage returns defaultedLanguage
	// But since no msgHandler, getErrorMsg returns the raw message
	xerr := xerror.NewXCode(
		xcode.NewWithHTTPStatus(10001, http.StatusBadRequest, "raw message"),
	)
	r.WithError(xerr)

	var body map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &body)
	assert.Nil(t, err)
	assert.Equal(t, "raw message", body["message"])
}

// --- rebuildRequestBody with cached body ---

func TestRebuildRequestBody_WithCachedBody(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/test", bytes.NewBufferString(`{"key":"value"}`))

	// Test that WithError works correctly even when httpcontext is not set up.
	// rebuildRequestBody gracefully handles the case where MustParse returns an error.
	r := NewResponse(c)
	r.WithError(errors.New("test error"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
