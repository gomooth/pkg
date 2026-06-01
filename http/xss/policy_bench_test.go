package xss

import (
	"strings"
	"testing"
)

// ============================================================
// P2: DefaultStrictPolicy — 严格过滤策略 Sanitize
// ============================================================

func BenchmarkStrictPolicy_Sanitize_PlainText(b *testing.B) {
	b.ReportAllocs()

	policy := DefaultStrictPolicy()
	input := "Hello World! This is a safe plain text message."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.Sanitize(input)
	}
}

func BenchmarkStrictPolicy_Sanitize_Script(b *testing.B) {
	b.ReportAllocs()

	policy := DefaultStrictPolicy()
	input := `<script>alert('xss')</script><p>Safe content</p>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.Sanitize(input)
	}
}

func BenchmarkStrictPolicy_Sanitize_MixedHTML(b *testing.B) {
	b.ReportAllocs()

	policy := DefaultStrictPolicy()
	input := `<div onclick="evil()"><script>alert(1)</script><p>Hello</p><iframe src="evil.html"></iframe></div>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.Sanitize(input)
	}
}

func BenchmarkStrictPolicy_Sanitize_LongContent(b *testing.B) {
	b.ReportAllocs()

	policy := DefaultStrictPolicy()
	// 构建较长内容
	var sb strings.Builder
	sb.WriteString("<div>")
	for i := 0; i < 100; i++ {
		sb.WriteString("<p>Paragraph with <script>alert('xss')</script> content</p>")
	}
	sb.WriteString("</div>")
	input := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.Sanitize(input)
	}
}

// ============================================================
// P2: DefaultUGCPolicy — UGC 过滤策略 Sanitize
// ============================================================

func BenchmarkUGCPolicy_Sanitize_SafeHTML(b *testing.B) {
	b.ReportAllocs()

	policy := DefaultUGCPolicy()
	input := "<p>Hello <b>World</b>! <a href=\"https://example.com\">Link</a></p>"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.Sanitize(input)
	}
}

func BenchmarkUGCPolicy_Sanitize_MixedHTML(b *testing.B) {
	b.ReportAllocs()

	policy := DefaultUGCPolicy()
	input := `<p>Safe</p><script>alert('xss')</script><a href="javascript:evil()">Click</a><iframe src="evil.html"></iframe>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.Sanitize(input)
	}
}

func BenchmarkUGCPolicy_Sanitize_LongContent(b *testing.B) {
	b.ReportAllocs()

	policy := DefaultUGCPolicy()
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("<p>Paragraph with <em>emphasis</em> and <a href=\"https://example.com\">links</a></p>")
	}
	input := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = policy.Sanitize(input)
	}
}

// ============================================================
// P2: 策略创建开销
// ============================================================

func BenchmarkDefaultStrictPolicy(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultStrictPolicy()
	}
}

func BenchmarkDefaultUGCPolicy(b *testing.B) {
	b.ReportAllocs()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DefaultUGCPolicy()
	}
}
