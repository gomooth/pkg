package dbrepo

import (
	"context"
	"errors"

	"github.com/save95/xerror"
	"github.com/save95/xerror/xcode"
	"gorm.io/gorm"
)

// IDAO 数据访问对象接口，提供通用的CRUD操作
type IDAO[T any] interface {
	// Create 创建记录
	Create(ctx context.Context, record *T) error
	// Creates 批量创建记录
	Creates(ctx context.Context, records []*T) error
	// Save 保存记录（更新或创建）
	Save(ctx context.Context, record *T) error
	// First 根据ID查询单条记录
	First(ctx context.Context, id uint) (*T, error)
	// FirstWith 根据ID查询单条记录（支持预加载）
	FirstWith(ctx context.Context, id uint, preloads ...string) (*T, error)
	// Delete 软删除记录
	Delete(ctx context.Context, id uint) error
	// Remove 硬删除记录
	Remove(ctx context.Context, id uint) error
	// Count 统计记录数量
	Count(ctx context.Context) (int64, error)
	// Exists 判断记录是否存在
	Exists(ctx context.Context, id uint) (bool, error)
	// Update 更新记录
	Update(ctx context.Context, id uint, updates map[string]interface{}) error
}

// dao 数据访问对象，提供通用的CRUD操作
type dao[T any] struct {
	db *gorm.DB
}

// NewDAO 创建DAO实例
func NewDAO[T any](db *gorm.DB) IDAO[T] {
	return &dao[T]{db: db}
}

// Create 创建记录
func (d *dao[T]) Create(ctx context.Context, record *T) error {
	if record == nil {
		return xerror.WithXCode(xcode.DBRequestParamError)
	}
	if err := d.db.WithContext(ctx).Create(record).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// Creates 批量创建记录
func (d *dao[T]) Creates(ctx context.Context, records []*T) error {
	if len(records) == 0 {
		return xerror.WithXCode(xcode.DBRequestParamError)
	}

	// 检查是否有nil记录
	for i, record := range records {
		if record == nil {
			return xerror.WithXCodeMessagef(xcode.DBRequestParamError, "record at index %d is nil", i)
		}
	}

	if err := d.db.WithContext(ctx).CreateInBatches(records, 100).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// Save 保存记录（更新或创建）
func (d *dao[T]) Save(ctx context.Context, record *T) error {
	if record == nil {
		return xerror.WithXCode(xcode.DBRequestParamError)
	}
	if err := d.db.WithContext(ctx).Save(record).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// First 根据ID查询单条记录
func (d *dao[T]) First(ctx context.Context, id uint) (*T, error) {
	if id == 0 {
		return nil, xerror.WithXCode(xcode.DBRequestParamError)
	}

	var record T
	if err := d.db.WithContext(ctx).Where("id = ?", id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerror.WithXCode(xcode.DBRecordNotFound)
		}
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return &record, nil
}

// FirstWith 根据ID查询单条记录
func (d *dao[T]) FirstWith(ctx context.Context, id uint, preloads ...string) (*T, error) {
	if id == 0 {
		return nil, xerror.WithXCode(xcode.DBRequestParamError)
	}

	db := d.db.WithContext(ctx).Where("id = ?", id)
	for _, preload := range preloads {
		db = db.Preload(preload)
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

// Delete 软删除记录
func (d *dao[T]) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return xerror.WithXCode(xcode.DBRequestParamError)
	}

	model := new(T)
	if err := d.db.WithContext(ctx).Where("id = ?", id).Delete(model).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// Remove 硬删除记录
func (d *dao[T]) Remove(ctx context.Context, id uint) error {
	if id == 0 {
		return xerror.WithXCode(xcode.DBRequestParamError)
	}

	model := new(T)
	if err := d.db.Unscoped().Where("id = ?", id).Delete(model).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// Count 统计记录数量
func (d *dao[T]) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := d.db.WithContext(ctx).Model(new(T)).Count(&count).Error; err != nil {
		return 0, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return count, nil
}

// Exists 判断记录是否存在
func (d *dao[T]) Exists(ctx context.Context, id uint) (bool, error) {
	var count int64
	if err := d.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return count > 0, nil
}

// Update 更新记录
func (d *dao[T]) Update(ctx context.Context, id uint, updates map[string]interface{}) error {
	if err := d.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Updates(updates).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}
