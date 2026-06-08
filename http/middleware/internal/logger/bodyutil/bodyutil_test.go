package bodyutil

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReadBody_NormalBody 正常读取请求体
func TestReadBody_NormalBody(t *testing.T) {
	body := `{"key":"value"}`
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(body))

	result := ReadBody(req)
	assert.Equal(t, []byte(body), result)

	// Body 应可再次读取
	body2, err := io.ReadAll(req.Body)
	assert.NoError(t, err)
	assert.Equal(t, body, string(body2))
}

// TestReadBody_NilBody 请求体为 nil
func TestReadBody_NilBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)

	result := ReadBody(req)
	assert.Nil(t, result)
}

// TestReadBody_EmptyBody 请求体为空
func TestReadBody_EmptyBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(""))

	result := ReadBody(req)
	assert.Empty(t, result)
}

// TestReadBody_LargeBody 大请求体被截断到 maxBodySize
func TestReadBody_LargeBody(t *testing.T) {
	// 创建超过 10MB 的请求体
	largeBody := strings.Repeat("a", 10<<20+100)
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(largeBody))

	result := ReadBody(req)
	assert.Equal(t, 10<<20, len(result), "body should be truncated to maxBodySize")
}

// TestReadBody_BinaryBody 二进制请求体
func TestReadBody_BinaryBody(t *testing.T) {
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF}
	req, _ := http.NewRequest(http.MethodPost, "/test", bytes.NewReader(binaryData))

	result := ReadBody(req)
	assert.Equal(t, binaryData, result)
}

// TestReadBody_BodyReconstructed 读取后 Body 被重建可再次读取
func TestReadBody_BodyReconstructed(t *testing.T) {
	originalBody := "hello world"
	req, _ := http.NewRequest(http.MethodPost, "/test", strings.NewReader(originalBody))

	ReadBody(req)

	// 验证 Body 可再次读取
	body2, err := io.ReadAll(req.Body)
	assert.NoError(t, err)
	assert.Equal(t, originalBody, string(body2))
}
