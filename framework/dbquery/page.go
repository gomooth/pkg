package dbquery

import (
	"encoding/json"
	"fmt"

	"github.com/gomooth/pkg/framework/pager"
)

// IPageSpec 定义查询的分页维度。
// 独立于过滤和排序，明确区分偏移量分页和游标分页两种模式。
type IPageSpec interface {
	// String 生成分页部分的缓存键
	String() string
}

// OffsetPage 偏移量分页（传统 LIMIT/OFFSET）
type OffsetPage struct {
	Offset int
	Limit  int
}

func (p OffsetPage) String() string {
	bs, _ := json.Marshal(map[string]int{"offset": p.Offset, "limit": p.Limit})
	return string(bs)
}

// CursorPageSpec 游标分页（WHERE cursor > value + LIMIT）。
// 封装游标参数 + 列白名单，将散落在 pager（CursorPage 类型）、
// gormbuild（cursorFields 白名单）和 searcher（column 参数）中的游标知识收敛为一处。
type CursorPageSpec struct {
	Page   pager.CursorPage // 游标分页参数（游标值、方向、每页条数）
	Column string           // 游标对应的数据库列名
	Fields map[string]string // 游标列白名单：逻辑名 → 数据库列名
}

func (p *CursorPageSpec) String() string {
	type cursorJSON struct {
		Value     string `json:"value"`
		Direction int    `json:"direction"`
		Limit     int    `json:"limit"`
		Column    string `json:"column"`
	}
	data := cursorJSON{
		Value:     p.Page.Value,
		Direction: int(p.Page.Direction),
		Limit:     p.Page.Limit,
		Column:    p.Column,
	}
	bs, _ := json.Marshal(data)
	return string(bs)
}

// IsCursor 判断分页规格是否为游标分页
func IsCursor(p IPageSpec) bool {
	_, ok := p.(*CursorPageSpec)
	return ok
}

// PageOf 从 IQuery 中提取偏移量分页参数。
// 如果不是偏移量分页或无分页，返回 (0, DefaultPageSize, false)。
func PageOf[F any](q IQuery[F]) (offset, limit int, ok bool) {
	if q == nil || q.Page() == nil {
		return 0, pager.DefaultPageSize, false
	}
	if p, isOffset := q.Page().(OffsetPage); isOffset {
		return p.Offset, p.Limit, true
	}
	return 0, pager.DefaultPageSize, false
}

// CursorPageOf 从 IQuery 中提取游标分页参数。
// 如果不是游标分页，返回 nil。
func CursorPageOf[F any](q IQuery[F]) *CursorPageSpec {
	if q == nil || q.Page() == nil {
		return nil
	}
	if p, ok := q.Page().(*CursorPageSpec); ok {
		return p
	}
	return nil
}

// PaginateValues 从 IQuery 中提取分页参数，供缓存键生成使用。
// 返回 (start, limit, isPaginated)。
func PaginateValues[F any](q IQuery[F]) (start, limit int, paginated bool) {
	if q == nil || q.Page() == nil {
		return 0, 0, false
	}
	switch p := q.Page().(type) {
	case OffsetPage:
		return p.Offset, p.Limit, true
	case *CursorPageSpec:
		return 0, p.Page.Limit, true
	default:
		return 0, 0, false
	}
}

// CacheKeyPart 从 IQuery 生成分页部分的缓存键片段
func CacheKeyPart[F any](q IQuery[F]) string {
	if q == nil || q.Page() == nil {
		return ""
	}
	return q.Page().String()
}

// FormatPaginateKey 格式化分页缓存键的前缀
func FormatPaginateKey(name string, start, limit int, filterHash string) string {
	return fmt.Sprintf("%s:paginate:%d,%d:%s", name, start, limit, filterHash)
}

// FormatListKey 格式化列表缓存键
func FormatListKey(name, filterHash string) string {
	return fmt.Sprintf("%s:list:%s", name, filterHash)
}
