package dbquery

import (
	"fmt"
	"log/slog"

	"github.com/gomooth/pkg/framework/pager"
	"gorm.io/gorm"
)

// Build 将 IQuery 应用到 GORM DB 实例，一站式构建。
// 内部按顺序调用 ApplyFilter → ApplyPreloads → ApplySort → ApplyPage，
// 每个步骤也可独立调用。
func Build[F any](db *gorm.DB, q IQuery[F], opts ...BuildOption[F]) (*gorm.DB, error) {
	cfg := &buildConfig[F]{}
	for _, opt := range opts {
		opt(cfg)
	}

	// 1. 过滤条件
	db = ApplyFilter(db, q, cfg.filterTransfer)

	// 2. 预加载
	db = ApplyPreloads(db, q.Preloads())

	// 3. 排序
	sortSpec := q.Sort()
	if sortSpec != nil && len(sortSpec.Sorters()) > 0 {
		var err error
		db, err = ApplySort(db, sortSpec, cfg.sortMapping)
		if err != nil {
			return db, err
		}
	} else if cfg.sortMapping != nil {
		// 无用户排序时应用默认排序
		db = db.Order(cfg.sortMapping.DefaultSort())
	}

	// 4. 分页
	if q.Page() != nil && !cfg.skipPage {
		db = ApplyPage(db, q.Page())
	}

	return db, nil
}

// --- 可独立调用的分解步骤 ---

// ApplyFilter 应用过滤条件到 GORM 查询
func ApplyFilter[F any](db *gorm.DB, q IQuery[F], transfer func(*F, *gorm.DB) *gorm.DB) *gorm.DB {
	if transfer != nil && q.Filter() != nil {
		db = transfer(q.Filter(), db)
	}
	return db
}

// ApplyPreloads 应用预加载
func ApplyPreloads(db *gorm.DB, preloads []string) *gorm.DB {
	for _, p := range preloads {
		db = db.Preload(p)
	}
	return db
}

// ApplySort 应用排序规则，使用 SortMapping 做字段映射和校验
func ApplySort(db *gorm.DB, spec ISortSpec, mapping *SortMapping) (*gorm.DB, error) {
	if mapping == nil {
		mapping = NewSortMapping()
	}

	sorters := spec.Sorters()
	if len(sorters) == 0 {
		db = db.Order(mapping.DefaultSort())
		return db, nil
	}

	for _, s := range sorters {
		col, ok := mapping.Resolve(s.Field)
		if !ok {
			if mapping.IsStrict() {
				return db, fmt.Errorf("unknown sort field: %s", s.Field)
			}
			slog.Warn("dbquery: unknown sort field skipped",
				slog.String("component", "dbquery"),
				slog.String("field", s.Field),
			)
			continue
		}
		db = db.Order(fmt.Sprintf("%s %s", col, s.Sorted.String()))
	}
	return db, nil
}

// ApplyPage 应用分页（偏移量或游标）
func ApplyPage(db *gorm.DB, page IPageSpec) *gorm.DB {
	switch p := page.(type) {
	case OffsetPage:
		db = db.Offset(p.Offset).Limit(p.Limit)
	case *CursorPageSpec:
		if len(p.Page.Value) > 0 && len(p.Column) > 0 {
			col, ok := p.Fields[p.Column]
			if !ok {
				slog.Warn("dbquery: unknown cursor field skipped",
					slog.String("component", "dbquery"),
					slog.String("field", p.Column),
				)
				db = db.Limit(p.Page.Limit)
				return db
			}
			if p.Page.Direction == pager.CursorBefore {
				db = db.Where(col+" < ?", p.Page.Value).Order(col + " DESC")
			} else {
				db = db.Where(col+" > ?", p.Page.Value).Order(col + " ASC")
			}
		}
		db = db.Limit(p.Page.Limit)
	}
	return db
}

// --- BuildOption 和 buildConfig ---

// BuildOption Build 函数的配置选项
type BuildOption[F any] func(*buildConfig[F])

type buildConfig[F any] struct {
	filterTransfer func(*F, *gorm.DB) *gorm.DB
	sortMapping    *SortMapping
	skipPage       bool
}

// WithFilterTransfer 设置过滤条件转换函数
func WithFilterTransfer[F any](fn func(*F, *gorm.DB) *gorm.DB) BuildOption[F] {
	return func(c *buildConfig[F]) {
		c.filterTransfer = fn
	}
}

// WithSortMapping 设置排序字段映射
func WithSortMapping[F any](m *SortMapping) BuildOption[F] {
	return func(c *buildConfig[F]) {
		c.sortMapping = m
	}
}

// WithSkipPage 跳过分页应用，用于 Count 查询等不需要 offset/limit 的场景
func WithSkipPage[F any]() BuildOption[F] {
	return func(c *buildConfig[F]) {
		c.skipPage = true
	}
}
