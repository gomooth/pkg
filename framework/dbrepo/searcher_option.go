package dbrepo

import (
	"github.com/gomooth/pkg/framework/dbquery"
	"gorm.io/gorm"
)

// searcherOption Searcher 选项的中间结构体
type searcherOption[M any, F any] struct {
	filterTransfer  func(*F, *gorm.DB) *gorm.DB
	sortMapping     *dbquery.SortMapping
	cursorExtractor func(*M) string
}

// SearcherOption searcher 构造选项
type SearcherOption[M any, F any] func(*searcherOption[M, F])

// WithFilterTransfer 设置过滤条件转换函数
func WithFilterTransfer[M any, F any](transfer func(f *F, db *gorm.DB) *gorm.DB) SearcherOption[M, F] {
	return func(o *searcherOption[M, F]) {
		o.filterTransfer = transfer
	}
}

// WithSortMapping 设置排序字段映射，替代旧的 WithSortKeyMap
func WithSortMapping[M any, F any](m *dbquery.SortMapping) SearcherOption[M, F] {
	return func(o *searcherOption[M, F]) {
		o.sortMapping = m
	}
}

// WithCursorExtractor 设置游标值提取函数，用于 ListByCursor 从最后一条记录提取下一页游标
func WithCursorExtractor[M any, F any](fn func(*M) string) SearcherOption[M, F] {
	return func(o *searcherOption[M, F]) {
		o.cursorExtractor = fn
	}
}

type findOption struct {
	preloads []string
	selects  []string
	start    int
	limit    int
}

type findOptionBuilder func(*findOption)

// WithPreload 预加载关联
func WithPreload(preloads ...string) findOptionBuilder {
	return func(opt *findOption) {
		opt.preloads = preloads
	}
}

// WithSelect 设置查询字段选择，仅查询指定列
func WithSelect(selects ...string) findOptionBuilder {
	return func(opt *findOption) {
		if opt.selects == nil {
			opt.selects = make([]string, 0, len(selects))
		}
		opt.selects = append(opt.selects, selects...)
	}
}

// WithTx 返回绑定指定事务连接的 Searcher 实例
func (q *searcher[M, F]) WithTx(tx *gorm.DB) ISearcher[M, F] {
	if tx == nil {
		return q
	}
	return &searcher[M, F]{
		db:              tx,
		filterTransfer:  q.filterTransfer,
		sortMapping:     q.sortMapping,
		cursorExtractor: q.cursorExtractor,
	}
}
