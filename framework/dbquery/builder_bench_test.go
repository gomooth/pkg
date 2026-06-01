package dbquery

import (
	"testing"

	"github.com/gomooth/pkg/framework/pager"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ============================================================
// 辅助：构造基于 SQLite 内存的 GORM DB 实例
// ============================================================

type benchModel struct {
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
	if err := db.AutoMigrate(&benchModel{}); err != nil {
		b.Fatal(err)
	}
	return db
}

// benchFilter 及其 transfer 函数
type benchFilter struct {
	Name   string `json:"name"`
	Status int    `json:"status"`
}

func benchFilterTransfer(f *benchFilter, db *gorm.DB) *gorm.DB {
	if f.Name != "" {
		db = db.Where("name LIKE ?", "%"+f.Name+"%")
	}
	if f.Status > 0 {
		db = db.Where("status = ?", f.Status)
	}
	return db
}

// ============================================================
// BenchmarkBuild — 查询构建，每次列表请求必经
// ============================================================

func BenchmarkBuild(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	sortMapping := NewSortMapping(
		WithSortFields("id", "name", "created_at", "status"),
		WithDefaultSort("id"),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := NewQuery(benchFilter{Name: "test", Status: 1},
			WithSorts[benchFilter]("-created_at,+name"),
			WithOffsetPage[benchFilter](0, 20),
		)
		_, _ = Build(db.Model(&benchModel{}), q,
			WithFilterTransfer(benchFilterTransfer),
			WithSortMapping[benchFilter](sortMapping),
		)
	}
}

func BenchmarkBuild_WithCursorPage(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	sortMapping := NewSortMapping(
		WithSortFields("id", "name", "created_at", "status"),
		WithDefaultSort("id"),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q := NewQuery(benchFilter{Name: "test"},
			WithSorts[benchFilter]("-id"),
			WithCursorPage[benchFilter](pager.CursorPage{
				Value:     "100",
				Direction: pager.CursorAfter,
				Limit:     20,
			}, "id", map[string]string{"id": "id", "created_at": "created_at"}),
		)
		_, _ = Build(db.Model(&benchModel{}), q,
			WithFilterTransfer(benchFilterTransfer),
			WithSortMapping[benchFilter](sortMapping),
		)
	}
}

// ============================================================
// BenchmarkApplySort — SortMapping.Resolve + Order 生成
// ============================================================

func BenchmarkApplySort_SingleField(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	mapping := NewSortMapping(
		WithSortFields("id", "name", "created_at", "status"),
		WithDefaultSort("id"),
	)
	spec := NewSortSpec([]pager.Sorter{
		{Field: "created_at", Sorted: pager.DESC},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplySort(db.Model(&benchModel{}), spec, mapping)
	}
}

func BenchmarkApplySort_MultiField(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	mapping := NewSortMapping(
		WithSortFields("id", "name", "created_at", "status", "updated_at", "priority"),
		WithDefaultSort("id"),
	)
	spec := NewSortSpec([]pager.Sorter{
		{Field: "status", Sorted: pager.ASC},
		{Field: "priority", Sorted: pager.DESC},
		{Field: "created_at", Sorted: pager.DESC},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplySort(db.Model(&benchModel{}), spec, mapping)
	}
}

func BenchmarkApplySort_KeyMap(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	mapping := NewSortMapping(
		WithSortKeyMap(map[string]string{
			"Name":      "user_name",
			"CreatedAt": "created_at",
			"Status":    "user_status",
		}),
		WithDefaultSort("created_at"),
	)
	spec := NewSortSpec([]pager.Sorter{
		{Field: "name", Sorted: pager.ASC},
		{Field: "status", Sorted: pager.DESC},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplySort(db.Model(&benchModel{}), spec, mapping)
	}
}

func BenchmarkApplySort_DefaultSort(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	mapping := NewSortMapping(
		WithSortFields("id", "created_at"),
		WithDefaultSort("id"),
	)
	spec := NewSortSpec(nil) // 无排序 → 使用默认排序

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplySort(db.Model(&benchModel{}), spec, mapping)
	}
}

// ============================================================
// BenchmarkApplyPage — 游标分页 / 偏移量分页构建
// ============================================================

func BenchmarkApplyPage_Offset(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	page := OffsetPage{Offset: 100, Limit: 20}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ApplyPage(db.Model(&benchModel{}), page)
	}
}

func BenchmarkApplyPage_CursorAfter(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	page := &CursorPageSpec{
		Page: pager.CursorPage{
			Value:     "2024-01-15T10:30:00Z",
			Direction: pager.CursorAfter,
			Limit:     20,
		},
		Column: "created_at",
		Fields: map[string]string{
			"id":         "id",
			"created_at": "created_at",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ApplyPage(db.Model(&benchModel{}), page)
	}
}

func BenchmarkApplyPage_CursorBefore(b *testing.B) {
	b.ReportAllocs()

	db := newBenchDB(b)
	page := &CursorPageSpec{
		Page: pager.CursorPage{
			Value:     "2024-01-15T10:30:00Z",
			Direction: pager.CursorBefore,
			Limit:     20,
		},
		Column: "created_at",
		Fields: map[string]string{
			"id":         "id",
			"created_at": "created_at",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ApplyPage(db.Model(&benchModel{}), page)
	}
}

// ============================================================
// BenchmarkQuery_String — 缓存键生成
// ============================================================

func BenchmarkQuery_String(b *testing.B) {
	b.ReportAllocs()

	q := NewQuery(benchFilter{Name: "test", Status: 1},
		WithSorts[benchFilter]("-created_at,+name"),
		WithOffsetPage[benchFilter](0, 20),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = q.String()
	}
}

// ============================================================
// BenchmarkHashKey — FNV-1a 哈希
// ============================================================

func BenchmarkHashKey(b *testing.B) {
	b.ReportAllocs()

	key := `{"filter":{"name":"test","status":1},"sort":"ASC","page":"{\"offset\":0,\"limit\":20}"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = HashKey(key)
	}
}
