package dbquery

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
)

// IQuery 组合过滤、排序、分页三个维度，表示完整的查询规格。
// 替代旧版 dbfilter.IFilter[F]，明确表达"这是一个查询，不只是一个过滤器"。
type IQuery[F any] interface {
	IFilter[F]

	// Sort 返回排序规格，nil 表示不排序
	Sort() ISortSpec
	// Page 返回分页规格，nil 表示不分页
	Page() IPageSpec
	// String 生成组合缓存键（filter + sort + page 三维序列化）
	String() string
}

// Query IQuery 的默认实现
type Query[F any] struct {
	filter   *F
	preloads []string
	sort     ISortSpec
	page     IPageSpec
}

// NewQuery 创建查询规格
func NewQuery[F any](filter F, opts ...func(*queryOption[F])) IQuery[F] {
	o := &queryOption[F]{}
	for _, opt := range opts {
		opt(o)
	}
	return &Query[F]{
		filter:   &filter,
		preloads: o.preloads,
		sort:     o.sort,
		page:     o.page,
	}
}

func (q *Query[F]) Filter() *F {
	if q.filter == nil {
		q.filter = new(F)
	}
	return q.filter
}

func (q *Query[F]) Preloads() []string {
	if q.preloads == nil {
		return []string{}
	}
	return q.preloads
}

func (q *Query[F]) Sort() ISortSpec {
	return q.sort
}

func (q *Query[F]) Page() IPageSpec {
	return q.page
}

// queryJSON 缓存键序列化结构
type queryJSON[F any] struct {
	Filter   any    `json:"filter"`
	Sort     string `json:"sort,omitempty"`
	Page     string `json:"page,omitempty"`
	Preloads []string `json:"preloads,omitempty"`
}

func (q *Query[F]) String() string {
	// 排序 preloads 保证缓存 key 稳定
	preloads := q.Preloads()
	sortedPreloads := make([]string, len(preloads))
	copy(sortedPreloads, preloads)
	sort.Strings(sortedPreloads)

	data := queryJSON[F]{
		Filter:   q.Filter(),
		Sort:     sortString(q.sort),
		Page:     pageString(q.page),
		Preloads: sortedPreloads,
	}
	bs, _ := json.Marshal(data)
	return string(bs)
}

// HashKey 使用 FNV-1a 128位哈希生成缓存键
func HashKey(s string) string {
	h := fnv.New128a()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func sortString(s ISortSpec) string {
	if s == nil {
		return ""
	}
	return s.String()
}

func pageString(p IPageSpec) string {
	if p == nil {
		return ""
	}
	return p.String()
}
