package xss

import "github.com/microcosm-cc/bluemonday"

// DefaultStrictPolicy 返回严格过滤策略（过滤所有HTML元素及属性）
func DefaultStrictPolicy() *bluemonday.Policy {
	return bluemonday.StrictPolicy()
}

// DefaultUGCPolicy 返回 UGC 过滤策略（过滤不安全的HTML，保留安全的）
func DefaultUGCPolicy() *bluemonday.Policy {
	return bluemonday.UGCPolicy()
}
