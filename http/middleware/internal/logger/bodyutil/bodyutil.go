package bodyutil

import (
	"bytes"
	"io"
	"net/http"
)

const maxBodySize = 10 << 20 // 10MB

// ReadBody 读取请求体，同时重建 Body 使其可被再次读取。
// 返回读取到的字节数组。如果请求体为 nil 或读取失败，返回 nil。
func ReadBody(r *http.Request) []byte {
	if r.Body == nil {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	_ = r.Body.Close()

	if err != nil {
		return nil
	}

	// 重建 Body 以便后续读取
	r.Body = io.NopCloser(bytes.NewBuffer(body))
	return body
}
