package pager

import (
	"testing"
)

// ============================================================
// P2: SanitizePageSize — 分页大小校正
// ============================================================

func BenchmarkSanitizePageSize(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizePageSize(0)
		_ = SanitizePageSize(20)
		_ = SanitizePageSize(-1)
		_ = SanitizePageSize(600)
	}
}

func BenchmarkSanitizePageSize_Default(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizePageSize(0)
	}
}

func BenchmarkSanitizePageSize_Valid(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizePageSize(20)
	}
}

func BenchmarkSanitizePageSize_OverMax(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SanitizePageSize(1000)
	}
}

// ============================================================
// P2: ParseSorts — 排序解析
// ============================================================

func BenchmarkParseSorts_Empty(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseSorts("")
	}
}

func BenchmarkParseSorts_SingleField(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseSorts("-created_at")
	}
}

func BenchmarkParseSorts_MultiField(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseSorts("-created_at,+name,status")
	}
}

func BenchmarkParseSorts_Complex(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseSorts("-created_at,+name,status,*priority,-updated_at,+id")
	}
}

// ============================================================
// P2: Sorted.String — 排序方向序列化
// ============================================================

func BenchmarkSorted_String(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ASC.String()
		_ = DESC.String()
		_ = Custom.String()
	}
}
