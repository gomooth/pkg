package restful

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ============================================================
// 辅助：构造 gin.Context
// ============================================================

func newBenchGinContext(b *testing.B) *gin.Context {
	b.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/users?start=0&limit=20", nil)
	return c
}

// ============================================================
// P1: Retrieve — 单资源响应序列化
// ============================================================

func BenchmarkResponse_Retrieve(b *testing.B) {
	b.ReportAllocs()

	entity := map[string]any{
		"id":     1,
		"name":   "test-user",
		"status": 1,
		"email":  "test@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.Retrieve(entity)
	}
}

func BenchmarkResponse_Retrieve_Nil(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.Retrieve(nil)
	}
}

// ============================================================
// P1: ListWithPagination — 分页列表响应
// ============================================================

func BenchmarkResponse_ListWithPagination(b *testing.B) {
	b.ReportAllocs()

	items := make([]map[string]any, 20)
	for i := range items {
		items[i] = map[string]any{
			"id":   i + 1,
			"name": "user-" + string(rune('A'+i%26)),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.ListWithPagination(100, items)
	}
}

// ============================================================
// P1: Post — 新增资源响应
// ============================================================

func BenchmarkResponse_Post(b *testing.B) {
	b.ReportAllocs()

	entity := map[string]any{
		"id":   1,
		"name": "new-user",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.Post(entity)
	}
}

// ============================================================
// P1: WithMessage — 文本消息响应
// ============================================================

func BenchmarkResponse_WithMessage(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.WithMessage("operation succeeded")
	}
}

// ============================================================
// P1: WithError — 错误响应
// ============================================================

func BenchmarkResponse_WithError(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.WithError(http.ErrMissingFile)
	}
}

// ============================================================
// P1: Delete — 删除响应
// ============================================================

func BenchmarkResponse_Delete(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.Delete(nil)
	}
}

// ============================================================
// P1: SetHeader — 请求头设置
// ============================================================

func BenchmarkResponse_SetHeader(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.SetHeader("X-Request-Id", "abc-123")
		r.SetHeader("X-Trace-Id", "trace-456")
	}
}

// ============================================================
// P1: ListWithMoreFlag — 带更多标志的列表响应
// ============================================================

func BenchmarkResponse_ListWithMoreFlag(b *testing.B) {
	b.ReportAllocs()

	items := make([]map[string]any, 20)
	for i := range items {
		items[i] = map[string]any{
			"id":   i + 1,
			"name": "user-" + string(rune('A'+i%26)),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.ListWithMoreFlag(true, items)
	}
}

// ============================================================
// P1: ListWithCursor — 游标分页列表响应
// ============================================================

func BenchmarkResponse_ListWithCursor(b *testing.B) {
	b.ReportAllocs()

	items := make([]map[string]any, 20)
	for i := range items {
		items[i] = map[string]any{
			"id":   i + 1,
			"name": "user-" + string(rune('A'+i%26)),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.ListWithCursor("cursor_abc123", items)
	}
}

// ============================================================
// P1: TableWithPagination — 表格分页响应
// ============================================================

func BenchmarkResponse_TableWithPagination(b *testing.B) {
	b.ReportAllocs()

	resp := &TableResponse{
		TotalRow: 100,
		Columns:  []string{"name", "status"},
		RowKeys:  []string{"1", "2", "3"},
		Items: []*TableResponseItem{
			{Column: "name", RowKey: "1", Data: "Alice"},
			{Column: "status", RowKey: "1", Data: 1},
			{Column: "name", RowKey: "2", Data: "Bob"},
			{Column: "status", RowKey: "2", Data: 2},
			{Column: "name", RowKey: "3", Data: "Charlie"},
			{Column: "status", RowKey: "3", Data: 1},
		},
		Extends: []*TableResponseRowExtendItem{
			{RowKey: "1", Data: map[string]any{"editable": true}},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		r := NewResponse(c)
		r.TableWithPagination(resp)
	}
}

// ============================================================
// P1: NewResponse 构造开销
// ============================================================

func BenchmarkNewResponse(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := newBenchGinContext(b)
		_ = NewResponse(c)
	}
}
