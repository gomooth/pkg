package dbrepo

import (
	"context"
	"errors"

	"github.com/gomooth/pkg/framework/dbfilter"
	"github.com/gomooth/pkg/framework/dbquery/gormbuild"
	"github.com/save95/xerror"
	"github.com/save95/xerror/xcode"
	"gorm.io/gorm"
)

type ISearcher[M any, F any] interface {
	// All 查询所有记录
	All(ctx context.Context, option dbfilter.IFilter[F]) ([]*M, error)
	// List 分页查询记录（不返回总数）
	List(ctx context.Context, start, limit int, option dbfilter.IFilter[F]) ([]*M, error)
	// Paginate 分页查询记录（返回总数）
	Paginate(ctx context.Context, start, limit int, option dbfilter.IFilter[F]) ([]*M, uint, error)
	// CountBy 统计记录数量
	CountBy(ctx context.Context, filter *F) (int64, error)
	// ExistsBy 判断记录是否存在
	ExistsBy(ctx context.Context, filter *F) (bool, error)
	// Find 通用查询方法
	Find(ctx context.Context, option dbfilter.IFilter[F], optBuilders ...findOptionBuilder) ([]*M, error)
	// FirstWith 带选项查询单条记录
	FirstWith(ctx context.Context, option dbfilter.IFilter[F], optBuilders ...findOptionBuilder) (*M, error)
}

// searcher 查询构建器，提供通用的查询方法
type searcher[M any, F any] struct {
	db *gorm.DB

	filterTransfer func(filter *F, db *gorm.DB) *gorm.DB
	sortKeyMapping map[string]string
}

// NewSearcher 创建查询构建器
func NewSearcher[M any, F any](db *gorm.DB,
	filterTransfer func(filter *F, db *gorm.DB) *gorm.DB,
	sortKeyMapping map[string]string,
	opts ...builderOptionBuilder[M, F],
) ISearcher[M, F] {
	builder := &searcher[M, F]{
		db:             db,
		filterTransfer: filterTransfer,
		sortKeyMapping: sortKeyMapping,
	}

	for _, opt := range opts {
		opt(builder)
	}

	return builder
}

// All 查询所有记录
func (q *searcher[M, F]) All(ctx context.Context, option dbfilter.IFilter[F]) ([]*M, error) {
	db := q.db.WithContext(ctx).Model(new(M))
	db = gormbuild.Build(
		db,
		gormbuild.WithFilter(option, q.filterTransfer),
		gormbuild.WithSortKeyMappings[F](q.sortKeyMapping),
	)

	var records []*M
	if err := db.Find(&records).Error; err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return records, nil
}

// List 分页查询记录（不返回总数）
func (q *searcher[M, F]) List(ctx context.Context, start, limit int, option dbfilter.IFilter[F]) ([]*M, error) {
	db := q.db.WithContext(ctx).Model(new(M))
	db = gormbuild.Build(
		db,
		gormbuild.WithFilter(option, q.filterTransfer),
		gormbuild.WithSortKeyMappings[F](q.sortKeyMapping),
		gormbuild.WithPage[F](start, limit),
	)

	var records []*M
	if err := db.Find(&records).Error; err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return records, nil
}

// Paginate 分页查询记录（返回总数）
func (q *searcher[M, F]) Paginate(ctx context.Context, start, limit int, option dbfilter.IFilter[F]) ([]*M, uint, error) {
	db := q.db.WithContext(ctx).Model(new(M))
	db = gormbuild.Build(
		db,
		gormbuild.WithFilter(option, q.filterTransfer),
		gormbuild.WithSortKeyMappings[F](q.sortKeyMapping),
		gormbuild.WithPage[F](start, limit),
	)

	var total int64
	_ = db.Count(&total).Error

	var records []*M
	if err := db.Find(&records).Error; nil != err {
		return nil, 0, xerror.WrapWithXCode(err, xcode.DBFailed)
	}

	return records, uint(total), nil
}

// CountBy 统计记录数量
func (q *searcher[M, F]) CountBy(ctx context.Context, filter *F) (int64, error) {
	db := q.db.WithContext(ctx).Model(new(M))
	if q.filterTransfer != nil {
		db = q.filterTransfer(filter, db)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return count, nil
}

// ExistsBy 判断记录是否存在
func (q *searcher[M, F]) ExistsBy(ctx context.Context, filter *F) (bool, error) {
	count, err := q.CountBy(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Find 通用查询方法
func (q *searcher[M, F]) Find(ctx context.Context, option dbfilter.IFilter[F], optBuilders ...findOptionBuilder) ([]*M, error) {
	opt := new(findOption)
	for _, build := range optBuilders {
		build(opt)
	}

	db := q.db.WithContext(ctx).Model(new(M))
	db = gormbuild.Build(
		db,
		gormbuild.WithFilter(option, q.filterTransfer),
		gormbuild.WithSortKeyMappings[F](q.sortKeyMapping),
		gormbuild.WithPage[F](opt.start, opt.limit),
	)

	var records []*M
	if err := db.Find(&records).Error; err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return records, nil
}

// FirstWith 带选项查询单条记录
func (q *searcher[M, F]) FirstWith(ctx context.Context, option dbfilter.IFilter[F], optBuilders ...findOptionBuilder) (*M, error) {
	opt := new(findOption)
	for _, build := range optBuilders {
		build(opt)
	}

	db := q.db.WithContext(ctx).Model(new(M))
	db = gormbuild.Build(
		db,
		gormbuild.WithFilter(option, q.filterTransfer),
		gormbuild.WithSortKeyMappings[F](q.sortKeyMapping),
		gormbuild.WithPage[F](opt.start, opt.limit),
	)

	var record M
	if err := db.First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerror.WithXCode(xcode.DBRecordNotFound)
		}
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return &record, nil
}
