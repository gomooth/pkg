package dbrepo

import (
	"errors"

	"github.com/gomooth/pkg/framework/dbquery"
	"github.com/save95/xerror"
	"github.com/save95/xerror/xcode"
	"gorm.io/gorm"
)

// QueryOption 查询选项
type QueryOption func(*gorm.DB) *gorm.DB

// WithPreload 预加载关联
func WithPreload(preloads ...string) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		for _, preload := range preloads {
			db = db.Preload(preload)
		}
		return db
	}
}

// WithSelect 选择字段
func WithSelect(selects ...string) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Select(selects)
	}
}

// WithOrder 排序
func WithOrder(order string) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Order(order)
	}
}

// WithLimit 限制数量
func WithLimit(limit int) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Limit(limit)
	}
}

// WithOffset 偏移量
func WithOffset(offset int) QueryOption {
	return func(db *gorm.DB) *gorm.DB {
		return db.Offset(offset)
	}
}

// QueryBuilder 查询构建器，提供通用的查询方法
type QueryBuilder[T any, F any] struct {
	dao *DAO[T]
}

// NewQueryBuilder 创建查询构建器
func NewQueryBuilder[T any, F any](dao *DAO[T]) *QueryBuilder[T, F] {
	return &QueryBuilder[T, F]{dao: dao}
}

// All 查询所有记录
func (q *QueryBuilder[T, F]) All(option dbquery.IFilter[F]) ([]*T, error) {
	db := q.dao.DB().Model(q.dao.model)

	if option != nil {
		db = option.Build(db)
	}

	var records []*T
	if err := db.Find(&records).Error; err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return records, nil
}

// List 分页查询记录（不返回总数）
func (q *QueryBuilder[T, F]) List(start, limit int, option dbquery.IFilter[F]) ([]*T, error) {
	db := q.dao.DB().Model(q.dao.model)

	if option != nil {
		db = option.Build(db, dbquery.BuildWithPage(start, limit))
	} else {
		db = db.Offset(start).Limit(limit)
	}

	var records []*T
	if err := db.Find(&records).Error; err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return records, nil
}

// Paginate 分页查询记录（返回总数）
func (q *QueryBuilder[T, F]) Paginate(start, limit int, option dbquery.IFilter[F]) ([]*T, uint, error) {
	db := q.dao.DB().Model(q.dao.model)

	if option != nil {
		// 先计数
		var total int64
		countDB := option.Build(db)
		if err := countDB.Count(&total).Error; err != nil {
			return nil, 0, xerror.WrapWithXCode(err, xcode.DBFailed)
		}

		// 再查询数据
		db = option.Build(db, dbquery.BuildWithPage(start, limit))
		var records []*T
		if err := db.Find(&records).Error; err != nil {
			return nil, 0, xerror.WrapWithXCode(err, xcode.DBFailed)
		}
		return records, uint(total), nil
	}

	// 没有过滤条件的情况
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, xerror.WrapWithXCode(err, xcode.DBFailed)
	}

	var records []*T
	if err := db.Offset(start).Limit(limit).Find(&records).Error; err != nil {
		return nil, 0, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return records, uint(total), nil
}

// Count 统计记录数量
func (q *QueryBuilder[T, F]) Count(option dbquery.IFilter[F]) (int64, error) {
	db := q.dao.DB().Model(q.dao.model)

	if option != nil {
		db = option.Build(db)
	}

	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return count, nil
}

// Exists 判断记录是否存在
func (q *QueryBuilder[T, F]) Exists(option dbquery.IFilter[F]) (bool, error) {
	count, err := q.Count(option)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Find 通用查询方法
func (q *QueryBuilder[T, F]) Find(option dbquery.IFilter[F], queryOptions ...QueryOption) ([]*T, error) {
	db := q.dao.DB().Model(q.dao.model)

	if option != nil {
		db = option.Build(db)
	}

	// 应用查询选项
	for _, opt := range queryOptions {
		db = opt(db)
	}

	var records []*T
	if err := db.Find(&records).Error; err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return records, nil
}

// FirstWith 带选项查询单条记录
func (q *QueryBuilder[T, F]) FirstWith(option dbquery.IFilter[F], queryOptions ...QueryOption) (*T, error) {
	db := q.dao.DB().Model(q.dao.model)

	if option != nil {
		db = option.Build(db)
	}

	// 应用查询选项
	for _, opt := range queryOptions {
		db = opt(db)
	}

	var record T
	if err := db.First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerror.WithXCode(xcode.DBRecordNotFound)
		}
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return &record, nil
}
