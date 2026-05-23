package pager

// Cursor 游标值，表示上次查询的最后一条记录的排序键值（明文）
type Cursor string

// CursorDirection 游标翻页方向
type CursorDirection int

const (
	// CursorAfter 向后翻页（默认），查询排序值大于游标值的记录
	CursorAfter CursorDirection = iota
	// CursorBefore 向前翻页，查询排序值小于游标值的记录
	CursorBefore
)

// CursorPage 游标分页参数
type CursorPage struct {
	// Value 游标值，对应排序字段的值。为空时从头部开始查询
	Value string
	// Direction 翻页方向，默认 CursorAfter（向后）
	Direction CursorDirection
	// Limit 每页条数
	Limit int
}
