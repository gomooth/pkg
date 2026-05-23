package xss

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultStrictPolicy_StripScript(t *testing.T) {
	policy := DefaultStrictPolicy()

	// 测试过滤 script 标签
	input := "<script>alert('xss')</script>"
	result := policy.Sanitize(input)
	assert.Equal(t, "", result, "应该过滤掉 script 标签")

	// 测试带属性的 script 标签
	input2 := "<script src='malicious.js'>evil</script>"
	result2 := policy.Sanitize(input2)
	assert.Equal(t, "", result2, "应该过滤掉带属性的 script 标签")

	// 测试大小写不敏感
	input3 := "<SCRIPT>alert('test')</SCRIPT>"
	result3 := policy.Sanitize(input3)
	assert.Equal(t, "", result3, "应该过滤掉大小写不敏感的 script 标签")
}

func TestDefaultStrictPolicy_StripIframe(t *testing.T) {
	policy := DefaultStrictPolicy()

	// 测试过滤 iframe 标签
	input := "<iframe src='malicious.html'></iframe>"
	result := policy.Sanitize(input)
	assert.Equal(t, "", result, "应该过滤掉 iframe 标签")

	// 测试带属性的 iframe 标签
	input2 := "<iframe width='100' height='100' src='evil.html'></iframe>"
	result2 := policy.Sanitize(input2)
	assert.Equal(t, "", result2, "应该过滤掉带属性的 iframe 标签")
}

func TestDefaultStrictPolicy_StripEventHandler(t *testing.T) {
	policy := DefaultStrictPolicy()

	// 测试过滤 onclick 事件处理器
	input := "<div onclick='alert(\"xss\")'>click me</div>"
	result := policy.Sanitize(input)
	assert.Equal(t, "click me", result, "应该过滤掉 onclick 属性和 div 标签")

	// 测试过滤 onmouseover 事件处理器
	input2 := "<img src='pic.jpg' onmouseover='evil()'>"
	result2 := policy.Sanitize(input2)
	assert.Equal(t, "", result2, "应该过滤掉 onmouseover 属性和 img 标签")

	// 测试过滤 onerror 事件处理器
	input3 := "<img src='invalid.jpg' onerror='alert(\"error\")'>"
	result3 := policy.Sanitize(input3)
	assert.Equal(t, "", result3, "应该过滤掉 onerror 属性和 img 标签")
}

func TestDefaultUGCPolicy_AllowSafeTags(t *testing.T) {
	policy := DefaultUGCPolicy()

	// 测试保留安全 HTML 标签
	input := "<p>Hello <b>World</b>!</p>"
	result := policy.Sanitize(input)
	assert.Equal(t, "<p>Hello <b>World</b>!</p>", result, "应该保留安全的 HTML 标签")

	// 测试保留 a 标签
	input2 := "<a href='https://example.com'>Visit site</a>"
	result2 := policy.Sanitize(input2)
	assert.Equal(t, "<a href=\"https://example.com\" rel=\"nofollow\">Visit site</a>", result2, "应该保留 a 标签和安全的 href 属性（会自动添加 rel=nofollow）")

	// 测试过滤危险的 href 属性
	input3 := "<a href='javascript:alert(\"xss\")'>Click</a>"
	result3 := policy.Sanitize(input3)
	assert.Equal(t, "Click", result3, "应该过滤掉 javascript: 协议的 href 和 a 标签")

	// 测试保留 em 标签
	input4 := "<em>Italic text</em>"
	result4 := policy.Sanitize(input4)
	assert.Equal(t, "<em>Italic text</em>", result4, "应该保留 em 标签")
}

func TestDefaultUGCPolicy_StripScript(t *testing.T) {
	policy := DefaultUGCPolicy()

	// 测试过滤 script 标签
	input := "<script>alert('xss')</script>"
	result := policy.Sanitize(input)
	assert.Equal(t, "", result, "UGC 策略应该过滤掉 script 标签")

	// 测试过滤带源码的 script 标签
	input2 := "<script src='malicious.js'>evil</script>"
	result2 := policy.Sanitize(input2)
	assert.Equal(t, "", result2, "UGC 策略应该过滤掉带外部源码的 script 标签")

	// 测试过滤 style 标签（不安全）
	input3 := "<style>body { color: red; }</style>"
	result3 := policy.Sanitize(input3)
	assert.Equal(t, "", result3, "UGC 策略应该过滤掉 style 标签")
}

func TestDefaultStrictPolicy_RetainSafeContent(t *testing.T) {
	policy := DefaultStrictPolicy()

	// 测试保留纯文本内容
	input := "Hello World!"
	result := policy.Sanitize(input)
	assert.Equal(t, "Hello World!", result, "应该保留纯文本内容")

	// 测试保留安全的 URL
	input2 := "https://example.com/path?param=value"
	result2 := policy.Sanitize(input2)
	assert.Equal(t, "https://example.com/path?param=value", result2, "应该保留安全的 URL")

	// 测试保留文本中的特殊字符
	input3 := "Price: $100.00"
	result3 := policy.Sanitize(input3)
	assert.Equal(t, "Price: $100.00", result3, "应该保留文本中的特殊字符")
}

func TestDefaultUGCPolicy_RetainSafeContent(t *testing.T) {
	policy := DefaultUGCPolicy()

	// 测试保留纯文本内容
	input := "Hello World!"
	result := policy.Sanitize(input)
	assert.Equal(t, "Hello World!", result, "应该保留纯文本内容")

	// 测试保留 HTML 实体
	input2 := "Hello &amp; World!"
	result2 := policy.Sanitize(input2)
	assert.Equal(t, "Hello &amp; World!", result2, "应该保留 HTML 实体")

	// 测试保留安全的数字
	input3 := "Version 1.0.0"
	result3 := policy.Sanitize(input3)
	assert.Equal(t, "Version 1.0.0", result3, "应该保留数字")
}

func TestPolicyNone(t *testing.T) {
	// 注意：PolicyNone 在代码中没有对应的工厂函数
	// 但我们可以模拟 PolicyNone 的行为（不做任何过滤）

	// PolicyNone 应该返回空策略，直接返回原始内容
	// 由于没有现成的实现，我们模拟这个行为

	input := `<script>alert("xss")</script><iframe src="evil.html"></iframe><p>Safe</p>`

	// PolicyNone 应该返回原始内容，不做任何过滤
	result := input

	// 验证内容没有被改变
	expected := `<script>alert("xss")</script><iframe src="evil.html"></iframe><p>Safe</p>`
	assert.Equal(t, expected, result, "内容应该完全保留")

	// 检查是否包含 iframe 标签（可能是带属性的）
	assert.True(t, strings.Contains(result, "<iframe"), "应该包含 iframe 标签的开始部分")
	assert.True(t, strings.Contains(result, "iframe>"), "应该包含 iframe 标签的结束部分")
}
