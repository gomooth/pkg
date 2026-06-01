package dbrepo

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ============================================================
// 辅助：构造基于 SQLite 内存的 GORM DB 实例
// ============================================================

type benchUser struct {
	gorm.Model
	Name   string
	Status int
}

func newBenchDB(b *testing.B) *gorm.DB {
	b.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatal(err)
	}
	if err := db.AutoMigrate(&benchUser{}); err != nil {
		b.Fatal(err)
	}
	return db
}

// seedBenchDB 插入初始数据
func seedBenchDB(b *testing.B, db *gorm.DB, count int) {
	b.Helper()
	for i := 0; i < count; i++ {
		db.Create(&benchUser{Name: "user", Status: 1})
	}
}

// ============================================================
// P1: First — 根据 ID 查询单条记录，每次数据访问必经
// ============================================================

func BenchmarkDAO_First(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	seedBenchDB(b, db, 100)

	dao, err := NewDAO[benchUser](db)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := uint(i%100 + 1)
		_, _ = dao.First(ctx, id)
	}
}

// ============================================================
// P1: FirstWith — 带预加载的查询
// ============================================================

func BenchmarkDAO_FirstWith(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	seedBenchDB(b, db, 100)

	dao, err := NewDAO[benchUser](db)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := uint(i%100 + 1)
		_, _ = dao.FirstWith(ctx, id, "Profile")
	}
}

// ============================================================
// P1: Create — 创建记录
// ============================================================

func BenchmarkDAO_Create(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)

	dao, err := NewDAO[benchUser](db)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dao.Create(ctx, &benchUser{Name: "new-user", Status: 1})
	}
}

// ============================================================
// P1: Update — 更新指定字段
// ============================================================

func BenchmarkDAO_Update(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	seedBenchDB(b, db, 100)

	dao, err := NewDAO[benchUser](db)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := uint(i%100 + 1)
		_ = dao.Update(ctx, id, &benchUser{Name: "updated"}, "name")
	}
}

// ============================================================
// P1: Delete — 软删除
// ============================================================

func BenchmarkDAO_Delete(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)

	dao, err := NewDAO[benchUser](db)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	// 预创建足够多的记录
	total := b.N + 100
	for i := 0; i < total; i++ {
		db.Create(&benchUser{Name: "to-delete", Status: 1})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dao.Delete(ctx, uint(i+1))
	}
}

// ============================================================
// P1: WithTx — 返回绑定事务的 DAO 实例
// ============================================================

func BenchmarkDAO_WithTx(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)

	dao, err := NewDAO[benchUser](db)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dao.WithTx(db)
	}
}

// ============================================================
// P1: WithTx nil — 返回当前 DAO 实例
// ============================================================

func BenchmarkDAO_WithTx_Nil(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)

	dao, err := NewDAO[benchUser](db)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dao.WithTx(nil)
	}
}

// ============================================================
// P1: NewDAO — 构造开销
// ============================================================

func BenchmarkNewDAO(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewDAO[benchUser](db)
	}
}

func BenchmarkNewDAO_WithOptions(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewDAO[benchUser](db, WithBatchSize[benchUser](200))
	}
}

// ============================================================
// P1: Save — 保存记录
// ============================================================

func BenchmarkDAO_Save(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)

	dao, err := NewDAO[benchUser](db)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	// 先创建一条记录
	db.Create(&benchUser{Name: "initial", Status: 1})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dao.Save(ctx, &benchUser{
			Model:  gorm.Model{ID: 1},
			Name:   "saved",
			Status: 2,
		})
	}
}

// ============================================================
// P1: Creates — 批量创建
// ============================================================

func BenchmarkDAO_Creates(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)

	dao, err := NewDAO[benchUser](db)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	records := make([]*benchUser, 10)
	for i := range records {
		records[i] = &benchUser{Name: "batch-user", Status: i}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dao.Creates(ctx, records)
		// 清空以便重复插入（避免唯一约束冲突）
		for j := range records {
			records[j].ID = 0
			records[j].CreatedAt = time.Time{}
			records[j].UpdatedAt = time.Time{}
			records[j].DeletedAt = gorm.DeletedAt{}
		}
	}
}

// ============================================================
// P1: Remove — 硬删除
// ============================================================

func BenchmarkDAO_Remove(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)

	dao, err := NewDAO[benchUser](db)
	if err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()

	// 预创建
	total := b.N + 100
	for i := 0; i < total; i++ {
		db.Create(&benchUser{Name: "to-remove", Status: 1})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = dao.Remove(ctx, uint(i+1))
	}
}
