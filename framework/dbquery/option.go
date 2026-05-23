package dbquery

import (
	"github.com/gomooth/pkg/framework/pager"
)

// queryOption NewQuery 的配置选项
type queryOption[F any] struct {
	preloads []string
	sort     ISortSpec
	page     IPageSpec
}

// WithSorts 设置排序规则，格式同旧版："+name,-created_at,*custom"
func WithSorts[F any](sort string) func(*queryOption[F]) {
	return func(o *queryOption[F]) {
		sorters := pager.ParseSorts(sort)
		if len(sorters) > 0 {
			o.sort = NewSortSpec(sorters)
		}
	}
}

// WithSortSpec 设置排序规格（用于更精细的排序控制）
func WithSortSpec[F any](spec ISortSpec) func(*queryOption[F]) {
	return func(o *queryOption[F]) {
		o.sort = spec
	}
}

// WithPreloads 设置 GORM 预加载
func WithPreloads[F any](preload string, others ...string) func(*queryOption[F]) {
	return func(o *queryOption[F]) {
		o.preloads = append([]string{preload}, others...)
	}
}

// WithOffsetPage 设置偏移量分页
func WithOffsetPage[F any](start, limit int) func(*queryOption[F]) {
	return func(o *queryOption[F]) {
		if limit <= 0 {
			limit = pager.DefaultPageSize
		}
		limit = min(limit, pager.MaxPageSize)
		o.page = OffsetPage{Offset: start, Limit: limit}
	}
}

// WithOffsetPageMax 设置带最大条数限制的偏移量分页
func WithOffsetPageMax[F any](start, limit, maxLimit int) func(*queryOption[F]) {
	return func(o *queryOption[F]) {
		if limit <= 0 {
			limit = pager.DefaultPageSize
		}
		if maxLimit <= 0 {
			maxLimit = 100
		}
		limit = min(limit, maxLimit)
		o.page = OffsetPage{Offset: start, Limit: limit}
	}
}

// WithCursorPage 设置游标分页。
// cursorPage 游标分页参数（含游标值、方向、每页条数）。
// column 游标对应的数据库列名，如 "id"、"created_at"。
// fields 游标列白名单：逻辑名 → 数据库列名。
// 注意：column 必须在 fields 白名单中方可生效；未配置白名单时将跳过游标条件并输出警告。
func WithCursorPage[F any](cursorPage pager.CursorPage, column string, fields map[string]string) func(*queryOption[F]) {
	return func(o *queryOption[F]) {
		if cursorPage.Limit <= 0 {
			cursorPage.Limit = pager.DefaultPageSize
		}
		cursorPage.Limit = min(cursorPage.Limit, pager.MaxPageSize)
		o.page = &CursorPageSpec{
			Page:   cursorPage,
			Column: column,
			Fields: fields,
		}
	}
}
