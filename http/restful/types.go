package restful

const (
	// PageLinkHeaderKey 分页链接响应头键名
	PageLinkHeaderKey = "Link"
	// PageInfoHeaderKey 分页信息响应头键名
	PageInfoHeaderKey = "X-Pagination-Info"
	// TotalCountHeaderKey 总记录数响应头键名
	TotalCountHeaderKey = "X-PaginateTotal-Count"
	// HasMoreHeaderKey 是否有更多数据响应头键名
	HasMoreHeaderKey = "X-More-Resource"
	// NextCursorHeaderKey 下一页游标响应头键名
	NextCursorHeaderKey = "X-Next-Cursor"
	// ErrorCodeHeaderKey 错误码响应头键名
	ErrorCodeHeaderKey = "X-Error-Code"
	// ErrorDataHeaderKey 错误数据响应头键名
	ErrorDataHeaderKey = "X-Error-Data"
	// LangHeaderKey 语言响应头键名
	LangHeaderKey = "X-Language"
)

// IResourceResponse 资源 CRUD 响应
type IResourceResponse interface {
	// Retrieve 查询单个资源的响应
	Retrieve(entity any)
	// Post 新增请求的响应
	Post(entity any)
	// Put 全量更新资源的响应
	Put(entity any)
	// Patch 部分更新资源的响应
	// 部分 cdn 服务商不支持 http patch 方法，如 阿里云
	Patch(entity any)
	// Delete 删除的响应
	Delete(err error)
}

// IListResponse 列表响应
type IListResponse interface {
	// TableWithPagination 表格分页响应
	TableWithPagination(resp *TableResponse)
	// ListWithPagination 分页列表的响应
	ListWithPagination(totalRow uint, entities any)
	// ListWithMoreFlag 查询列表的响应
	ListWithMoreFlag(hasMore bool, entities any)
	// ListWithCursor 游标分页列表的响应，通过 X-Next-Cursor header 返回下一页游标
	ListWithCursor(nextCursor string, entities any)
}

// IMessageResponse 通用消息响应
type IMessageResponse interface {
	// WithMessage 通过 json 响应文本消息: {"message": "something..."}
	WithMessage(msg string)
	// WithBody 响应文本消息
	WithBody(body string)
	// WithError 响应错误消息(HttpStatus!=200)
	WithError(err error)
	// WithErrorData 响应错误消息(HttpStatus!=200)，并在 header 中返回错误数据
	WithErrorData(err error, data any)
}

// IResponse 组合接口，兼容需要全部能力的场景。
// SetHeader 返回 IResponse 维持链式调用。
type IResponse interface {
	IResourceResponse
	IListResponse
	IMessageResponse
	// SetHeader 设置请求头
	SetHeader(key, value string) IResponse
}

// TableResponse 表格分页响应数据结构
type TableResponse struct {
	TotalRow uint                          // 分页的记录条数
	Columns  []string                      // 表格列
	RowKeys  []string                      // 表格行
	Items    []*TableResponseItem          // 表格行数据
	Extends  []*TableResponseRowExtendItem // 表格行扩展数据
}

// TableResponseItem 表格单元格数据
type TableResponseItem struct {
	Column string // 列
	RowKey string // 行关键字
	Data   any    // 数据
}

// TableResponseRowExtendItem 表格行扩展数据
type TableResponseRowExtendItem struct {
	RowKey string // 行关键字
	Data   any    // 数据
}
