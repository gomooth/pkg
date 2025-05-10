package pager

// IPager 分页参数
type IPager[T any] interface {
	Filter() *T
	Sorters() []Sorter

	String() string
}

const (
	DefaultPageSize = 20
)
