package dbrepo

import (
	"context"
	"errors"
	"time"

	"github.com/gomooth/pkg/framework/telemetry"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"gorm.io/gorm"
)

var (
	dbRepoOperationCounter  metric.Int64Counter
	dbRepoOperationDuration metric.Float64Histogram
)

// daoOptionTarget DAO 选项的内部接口
type daoOptionTarget interface {
	setBatchSize(int)
}

// DAOOption DAO 选项函数（非泛型）
type DAOOption func(daoOptionTarget)

func init() {
	telemetry.OnProviderSet(func() {
		m := telemetry.Meter("dbrepo")
		dbRepoOperationCounter, _ = m.Int64Counter("dbrepo.operation")
		dbRepoOperationDuration, _ = m.Float64Histogram("dbrepo.operation.duration",
			metric.WithUnit("s"))
	})
}

func recordDBRepoMetric(ctx context.Context, component, operation string, dur time.Duration, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	attrs := metric.WithAttributes(
		attribute.String("component", component),
		attribute.String("operation", operation),
		attribute.String("result", result),
	)
	dbRepoOperationCounter.Add(ctx, 1, attrs)
	dbRepoOperationDuration.Record(ctx, dur.Seconds(), attrs)
}

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
	// Delete 软删除记录，返回影响行数
	Delete(ctx context.Context, id uint) (int64, error)
	// Remove 硬删除记录，返回影响行数
	Remove(ctx context.Context, id uint) (int64, error)
	// Update 更新指定字段（类型安全，显式声明要更新的字段名）
	Update(ctx context.Context, id uint, record *T, fields ...string) error
	// WithTx 返回绑定指定事务的 DAO 实例
	WithTx(tx *gorm.DB) IDAO[T]
}

// dao 数据访问对象，提供通用的CRUD操作
type dao[T any] struct {
	db        *gorm.DB
	batchSize int // 批量创建时的批次大小，默认 100
}

func (d *dao[T]) setBatchSize(size int) {
	if size > 0 {
		d.batchSize = size
	}
}

// NewDAO 创建DAO实例
// db 不能为 nil，否则返回错误
// opts 可选配置，如 WithBatchSize 设置批量创建时的批次大小
func NewDAO[T any](db *gorm.DB, opts ...DAOOption) (IDAO[T], error) {
	if db == nil {
		return nil, xerror.New("dbrepo: NewDAO called with nil *gorm.DB")
	}
	d := &dao[T]{db: db, batchSize: 100}
	for _, opt := range opts {
		opt(d)
	}
	return d, nil
}

// WithBatchSize 设置批量创建时的批次大小
func WithBatchSize(size int) DAOOption {
	return func(t daoOptionTarget) {
		t.setBatchSize(size)
	}
}

// Create 创建记录
func (d *dao[T]) Create(ctx context.Context, record *T) (err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "dao", "create", time.Since(start), err)
	}()

	if record == nil {
		return xerror.NewXCode(xcode.DBRequestParamError, "dao: record must not be nil")
	}
	if err := d.db.WithContext(ctx).Create(record).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// Creates 批量创建记录
func (d *dao[T]) Creates(ctx context.Context, records []*T) (err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "dao", "creates", time.Since(start), err)
	}()

	if len(records) == 0 {
		return xerror.NewXCode(xcode.DBRequestParamError, "dao: records must not be empty")
	}

	// 检查是否有nil记录
	for i, record := range records {
		if record == nil {
			return xerror.NewXCodef(xcode.DBRequestParamError, "record at index %d is nil", i)
		}
	}

	if err := d.db.WithContext(ctx).CreateInBatches(records, d.batchSize).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// Save 保存记录（更新或创建）
func (d *dao[T]) Save(ctx context.Context, record *T) (err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "dao", "save", time.Since(start), err)
	}()

	if record == nil {
		return xerror.NewXCode(xcode.DBRequestParamError, "dao: record must not be nil")
	}
	if err := d.db.WithContext(ctx).Save(record).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// First 根据ID查询单条记录
func (d *dao[T]) First(ctx context.Context, id uint) (record *T, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "dao", "first", time.Since(start), err)
	}()

	if id == 0 {
		return nil, xerror.NewXCode(xcode.DBRequestParamError, "dao: id must not be zero")
	}

	var r T
	if err := d.db.WithContext(ctx).Where("id = ?", id).First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerror.NewXCode(xcode.DBRecordNotFound)
		}
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return &r, nil
}

// FirstWith 根据ID查询单条记录
func (d *dao[T]) FirstWith(ctx context.Context, id uint, preloads ...string) (record *T, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "dao", "first_with", time.Since(start), err)
	}()

	if id == 0 {
		return nil, xerror.NewXCode(xcode.DBRequestParamError, "dao: id must not be zero")
	}

	db := d.db.WithContext(ctx).Where("id = ?", id)
	for _, preload := range preloads {
		db = db.Preload(preload)
	}

	var r T
	if err := db.First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerror.NewXCode(xcode.DBRecordNotFound)
		}
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}

	return &r, nil
}

// Delete 软删除记录，返回影响行数
func (d *dao[T]) Delete(ctx context.Context, id uint) (rowsAffected int64, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "dao", "delete", time.Since(start), err)
	}()

	if id == 0 {
		return 0, xerror.NewXCode(xcode.DBRequestParamError, "dao: id must not be zero")
	}

	model := new(T)
	result := d.db.WithContext(ctx).Where("id = ?", id).Delete(model)
	if result.Error != nil {
		return 0, xerror.WrapWithXCode(result.Error, xcode.DBFailed)
	}
	return result.RowsAffected, nil
}

// Remove 硬删除记录，返回影响行数
func (d *dao[T]) Remove(ctx context.Context, id uint) (rowsAffected int64, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "dao", "remove", time.Since(start), err)
	}()

	if id == 0 {
		return 0, xerror.NewXCode(xcode.DBRequestParamError, "dao: id must not be zero")
	}

	model := new(T)
	result := d.db.Unscoped().WithContext(ctx).Where("id = ?", id).Delete(model)
	if result.Error != nil {
		return 0, xerror.WrapWithXCode(result.Error, xcode.DBFailed)
	}
	return result.RowsAffected, nil
}

// Update 更新指定字段（类型安全，显式声明要更新的字段名）
// fields 为 GORM 列名，如 "name", "age"。使用 Select 显式指定字段，
// 可更新零值字段（如将 age 设为 0），避免 map[string]any 的类型不安全问题。
func (d *dao[T]) Update(ctx context.Context, id uint, record *T, fields ...string) (err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "dao", "update", time.Since(start), err)
	}()

	if id == 0 {
		return xerror.NewXCode(xcode.DBRequestParamError, "dao: id must not be zero")
	}
	if record == nil {
		return xerror.NewXCode(xcode.DBRequestParamError, "dao: record must not be nil")
	}
	if len(fields) == 0 {
		return xerror.NewXCode(xcode.DBRequestParamError, "dao: update fields must not be empty")
	}
	if err := d.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Select(fields).Updates(record).Error; err != nil {
		return xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return nil
}

// WithTx 返回绑定指定事务的 DAO 实例
// 若 tx 为 nil 则返回当前 DAO 实例（使用原始 DB 连接），避免后续操作 panic
func (d *dao[T]) WithTx(tx *gorm.DB) IDAO[T] {
	if tx == nil {
		return d
	}
	return &dao[T]{db: tx, batchSize: d.batchSize}
}
