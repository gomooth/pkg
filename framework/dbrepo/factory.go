package dbrepo

import (
	"context"

	"gorm.io/gorm"
)

// CreateDAO 创建基础DAO（兼容旧版本）
func CreateDAO[T any](model T, options ...interface{}) *DAO[T] {
	opts := make([]Option, 0)

	// 从选项中提取数据库连接
	for _, option := range options {
		if v, ok := option.(context.Context); ok {
			opts = append(opts, WithContext(v))
			continue
		}
		if v, ok := option.(*gorm.DB); ok {
			opts = append(opts, WithDB(v))
			continue
		}
	}

	return NewDAO(model, opts...)
}

// NewDAOWith 使用函数选项模式创建DAO（推荐使用）
func NewDAOWith[T any](model T, options ...Option) *DAO[T] {
	return NewDAO(model, options...)
}

// CreateQueryBuilder 创建查询构建器
func CreateQueryBuilder[T any, F any](dao *DAO[T]) *QueryBuilder[T, F] {
	return NewQueryBuilder[T, F](dao)
}

// NewQueryBuilderWith 使用函数选项模式创建查询构建器
func NewQueryBuilderWith[T any, F any](model T, options ...Option) *QueryBuilder[T, F] {
	dao := NewDAO(model, options...)
	return NewQueryBuilder[T, F](dao)
}

// WithTransaction 在事务中执行操作
func WithTransaction(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	return db.Transaction(fn)
}
