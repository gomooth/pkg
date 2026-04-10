package dbrepo

import (
	"errors"

	"github.com/save95/xerror"
	"github.com/save95/xerror/xcode"
	"gorm.io/gorm"
)

// DAO 数据访问对象，提供通用的CRUD操作
type DAO[T any] struct {
	db    *gorm.DB
	model T
}

// NewDAO 创建DAO实例
func NewDAO[T any](model T, options ...Option) *DAO[T] {
	config := &daoConfig{
		dbName: "platform", // 默认数据库名称
	}

	// 应用选项
	for _, opt := range options {
		opt(config)
	}

	// 获取数据库连接
	db := getDB(config)

	return &DAO[T]{db: db, model: model}
}

// DB 获取数据库连接
func (d *DAO[T]) DB() *gorm.DB {
	return d.db
}

// Create 创建记录
func (d *DAO[T]) Create(record *T) error {
	if record == nil {
		return xerror.WithXCode(xcode.DBRequestParamError)
	}
	if err := d.db.Create(record).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// Creates 批量创建记录
func (d *DAO[T]) Creates(records []*T) error {
	if len(records) == 0 {
		return nil
	}

	// 检查是否有nil记录
	for i, record := range records {
		if record == nil {
			return xerror.WithXCodeMessagef(xcode.DBRequestParamError, "record at index %d is nil", i)
		}
	}

	if err := d.db.CreateInBatches(records, 100).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// Save 保存记录（更新或创建）
func (d *DAO[T]) Save(record *T) error {
	if record == nil {
		return xerror.WithXCode(xcode.DBRequestParamError)
	}
	if err := d.db.Save(record).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// First 根据ID查询单条记录
func (d *DAO[T]) First(id uint) (*T, error) {
	var record T
	if err := d.db.Where("id = ?", id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerror.WithXCode(xcode.DBRecordNotFound)
		}
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return &record, nil
}

// FirstBy 根据指定字段查询单条记录
func (d *DAO[T]) FirstBy(field string, value interface{}) (*T, error) {
	var record T
	if err := d.db.Where(field+" = ?", value).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerror.WithXCode(xcode.DBRecordNotFound)
		}
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return &record, nil
}

// Delete 软删除记录
func (d *DAO[T]) Delete(id uint) error {
	if err := d.db.Where("id = ?", id).Delete(d.model).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// Remove 硬删除记录
func (d *DAO[T]) Remove(id uint) error {
	if err := d.db.Unscoped().Where("id = ?", id).Delete(d.model).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// Transaction 执行事务
func (d *DAO[T]) Transaction(fn func(tx *gorm.DB) error) error {
	return d.db.Transaction(fn)
}

// Count 统计记录数量
func (d *DAO[T]) Count() (int64, error) {
	var count int64
	if err := d.db.Model(d.model).Count(&count).Error; err != nil {
		return 0, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return count, nil
}

// Exists 判断记录是否存在
func (d *DAO[T]) Exists(id uint) (bool, error) {
	var count int64
	if err := d.db.Model(d.model).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return count > 0, nil
}

// Update 更新记录
func (d *DAO[T]) Update(id uint, updates map[string]interface{}) error {
	if err := d.db.Model(d.model).Where("id = ?", id).Updates(updates).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// FirstOrCreate 查找或创建记录
func (d *DAO[T]) FirstOrCreate(where map[string]interface{}, record *T) (*T, error) {
	if record == nil {
		return nil, xerror.WithXCode(xcode.DBRequestParamError)
	}
	if err := d.db.Where(where).FirstOrCreate(record).Error; err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return record, nil
}
