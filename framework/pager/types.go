package pager

// IPager 分页参数
type IPager[T any] interface {
	Filter() *T
	Sorters() []Sorter

	String() string
}

const (
	DefaultPageSize = 20
	MaxPageSize     = 500 // 最大分页大小限制
)

// SanitizePageSize 校正分页大小，确保在 [1, MaxPageSize] 范围内
// size <= 0 时返回 DefaultPageSize
func SanitizePageSize(size int) int {
	if size <= 0 {
		return DefaultPageSize
	}
	if size > MaxPageSize {
		return MaxPageSize
	}
	return size
}
