package httpmodel

// CursorSearchRequest 游标分页搜索请求
type CursorSearchRequest struct {
	// After 游标值，对应上次查询最后一条记录的排序键值。为空时从头部开始查询
	After string `form:"after"`
	// Limit 每页条数
	Limit int `form:"limit"`
}
