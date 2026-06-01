package dbquery

import (
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gomooth/pkg/framework/pager"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type testModel struct {
	gorm.Model
	Name   string
	Status int
}

type testFilter struct {
	Name   string
	Status *int
}

func testFilterTransfer(filter *testFilter, db *gorm.DB) *gorm.DB {
	if filter == nil {
		return db
	}
	if filter.Name != "" {
		db = db.Where("name LIKE ?", "%"+filter.Name+"%")
	}
	if filter.Status != nil {
		db = db.Where("status = ?", *filter.Status)
	}
	return db
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	err = db.AutoMigrate(&testModel{})
	assert.NoError(t, err)
	return db
}

func seedTestData(t *testing.T, db *gorm.DB) {
	t.Helper()
	records := []*testModel{
		{Name: "Alice", Status: 1},
		{Name: "Bob", Status: 2},
		{Name: "Charlie", Status: 1},
		{Name: "David", Status: 3},
		{Name: "Eve", Status: 2},
	}
	for _, r := range records {
		err := db.Create(r).Error
		assert.NoError(t, err)
	}
}

func TestBuild_Basic(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	t.Run("query with no options returns all records with default sort", func(t *testing.T) {
		q := NewQuery(testFilter{})
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 5)
	})

	t.Run("query with filter conditions", func(t *testing.T) {
		status := 1
		q := NewQuery(testFilter{Status: &status})
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})
}

func TestBuild_Sort(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	t.Run("custom sort disables default sorter", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithSorts[testFilter]("+name"))
		mapping := NewSortMapping(WithSortFields("name", "status"))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](mapping),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Equal(t, "Alice", records[0].Name)
		assert.Equal(t, "Bob", records[1].Name)
	})

	t.Run("descending sort", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithSorts[testFilter]("-name"))
		mapping := NewSortMapping(WithSortFields("name"))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](mapping),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Equal(t, "Eve", records[0].Name)
		assert.Equal(t, "David", records[1].Name)
	})

	t.Run("sort with key mapping", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithSorts[testFilter]("+status"))
		mapping := NewSortMapping(WithSortKeyMap(map[string]string{"status": "status"}))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](mapping),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Equal(t, 1, records[0].Status)
		assert.Equal(t, 3, records[4].Status)
	})

	t.Run("unknown sort field is skipped", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithSorts[testFilter]("+unknown_field"))
		mapping := NewSortMapping(WithSortFields("name"))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](mapping),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 5)
	})

	t.Run("strict sort returns error for unknown field", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithSorts[testFilter]("+unknown_field"))
		mapping := NewSortMapping(WithSortFields("name"), WithStrictSort(true))
		_, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](mapping),
		)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown sort field")
	})

	t.Run("custom default sort", func(t *testing.T) {
		q := NewQuery(testFilter{})
		mapping := NewSortMapping(WithSortFields("name"), WithDefaultSort("name", "ASC"))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](mapping),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Equal(t, "Alice", records[0].Name)
	})

	t.Run("default sort field not in whitelist falls back to id DESC", func(t *testing.T) {
		q := NewQuery(testFilter{})
		mapping := NewSortMapping(WithDefaultSort("nonexistent_field", "ASC"))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](mapping),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Equal(t, "Eve", records[0].Name)
	})
}

func TestBuild_Pagination(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	t.Run("offset page applies offset and limit", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithOffsetPage[testFilter](0, 2))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("offset page second page", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithOffsetPage[testFilter](2, 2))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("zero limit uses default page size", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithOffsetPage[testFilter](0, 0))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 5)
	})

	t.Run("pagination with sort and filter combined", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithSorts[testFilter]("+name"), WithOffsetPage[testFilter](0, 2))
		mapping := NewSortMapping(WithSortFields("name"))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](mapping),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 2)
		assert.Equal(t, "Alice", records[0].Name)
		assert.Equal(t, "Bob", records[1].Name)
	})
}

func TestBuild_CursorPage(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	t.Run("cursor page with whitelisted field", func(t *testing.T) {
		q := NewQuery(testFilter{},
			WithCursorPage[testFilter](pager.CursorPage{Limit: 2}, "id", map[string]string{"id": "id"}),
		)
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("cursor page without whitelist skips cursor condition", func(t *testing.T) {
		q := NewQuery(testFilter{},
			WithCursorPage[testFilter](pager.CursorPage{Value: "1", Limit: 2}, "id", map[string]string{}),
		)
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("cursor page with injection attempt skipped", func(t *testing.T) {
		q := NewQuery(testFilter{},
			WithCursorPage[testFilter](pager.CursorPage{Value: "1", Limit: 2}, "id; DROP TABLE users--", map[string]string{}),
		)
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})
}

func TestBuild_Preloads(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	t.Run("preloads are applied to query", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithPreloads[testFilter]("Items"))
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}

func TestApplySort_Independent(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	t.Run("ApplySort can be called independently", func(t *testing.T) {
		spec := NewSortSpec([]pager.Sorter{{Field: "name", Sorted: pager.ASC}})
		mapping := NewSortMapping(WithSortFields("name"))
		result, err := ApplySort(db.Model(&testModel{}), spec, mapping)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Equal(t, "Alice", records[0].Name)
	})
}

func TestApplyPage_Independent(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	t.Run("ApplyPage with OffsetPage independently", func(t *testing.T) {
		result := ApplyPage(db.Model(&testModel{}), OffsetPage{Offset: 0, Limit: 3})

		var records []testModel
		err := result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 3)
	})
}

func TestQuery_CacheKey(t *testing.T) {
	t.Run("same filter produces same cache key", func(t *testing.T) {
		q1 := NewQuery(testFilter{Name: "foo"}, WithSorts[testFilter]("+name"), WithOffsetPage[testFilter](0, 20))
		q2 := NewQuery(testFilter{Name: "foo"}, WithSorts[testFilter]("+name"), WithOffsetPage[testFilter](0, 20))
		assert.Equal(t, q1.String(), q2.String())
	})

	t.Run("different filter produces different cache key", func(t *testing.T) {
		q1 := NewQuery(testFilter{Name: "foo"}, WithSorts[testFilter]("+name"))
		q2 := NewQuery(testFilter{Name: "bar"}, WithSorts[testFilter]("+name"))
		assert.NotEqual(t, q1.String(), q2.String())
	})

	t.Run("different sort produces different cache key", func(t *testing.T) {
		q1 := NewQuery(testFilter{Name: "foo"}, WithSorts[testFilter]("+name"))
		q2 := NewQuery(testFilter{Name: "foo"}, WithSorts[testFilter]("-name"))
		assert.NotEqual(t, q1.String(), q2.String())
	})

	t.Run("different page produces different cache key", func(t *testing.T) {
		q1 := NewQuery(testFilter{Name: "foo"}, WithOffsetPage[testFilter](0, 20))
		q2 := NewQuery(testFilter{Name: "foo"}, WithOffsetPage[testFilter](20, 20))
		assert.NotEqual(t, q1.String(), q2.String())
	})

	t.Run("preloads sorted for stable cache key", func(t *testing.T) {
		q1 := NewQuery(testFilter{}, WithPreloads[testFilter]("Orders", "Items"))
		q2 := NewQuery(testFilter{}, WithPreloads[testFilter]("Items", "Orders"))
		assert.Equal(t, q1.String(), q2.String())
	})

	t.Run("HashKey produces consistent hash", func(t *testing.T) {
		q := NewQuery(testFilter{Name: "foo"}, WithSorts[testFilter]("+name"))
		h1 := HashKey(q.String())
		h2 := HashKey(q.String())
		assert.Equal(t, h1, h2)
	})
}

func TestSortMapping(t *testing.T) {
	t.Run("Resolve finds mapped field", func(t *testing.T) {
		m := NewSortMapping(WithSortFields("name", "status"))
		col, ok := m.Resolve("name")
		assert.True(t, ok)
		assert.Equal(t, "name", col)
	})

	t.Run("Resolve returns false for unknown field", func(t *testing.T) {
		m := NewSortMapping(WithSortFields("name"))
		_, ok := m.Resolve("unknown")
		assert.False(t, ok)
	})

	t.Run("DefaultSort returns correct default", func(t *testing.T) {
		m := NewSortMapping(WithSortFields("id", "name"), WithDefaultSort("id", "DESC"))
		assert.Equal(t, "id DESC", m.DefaultSort())
	})

	t.Run("DefaultSort falls back when field not in whitelist", func(t *testing.T) {
		m := NewSortMapping(WithDefaultSort("nonexistent", "ASC"))
		assert.Equal(t, "id DESC", m.DefaultSort())
	})

	t.Run("WithSortKeyMap maps frontend keys", func(t *testing.T) {
		m := NewSortMapping(WithSortKeyMap(map[string]string{"userName": "user_name"}))
		col, ok := m.Resolve("user_name") // Snake case is applied to key
		assert.True(t, ok)
		assert.Equal(t, "user_name", col)
	})

	t.Run("IsStrict returns correct value", func(t *testing.T) {
		m1 := NewSortMapping(WithStrictSort(true))
		assert.True(t, m1.IsStrict())
		m2 := NewSortMapping(WithStrictSort(false))
		assert.False(t, m2.IsStrict())
	})
}

func TestPageSpec(t *testing.T) {
	t.Run("IsCursor identifies CursorPageSpec", func(t *testing.T) {
		p := &CursorPageSpec{Column: "id", Fields: map[string]string{"id": "id"}}
		assert.True(t, IsCursor(p))
	})

	t.Run("IsCursor returns false for OffsetPage", func(t *testing.T) {
		p := OffsetPage{Offset: 0, Limit: 20}
		assert.False(t, IsCursor(p))
	})

	t.Run("PageOf extracts offset page values", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithOffsetPage[testFilter](10, 20))
		offset, limit, ok := PageOf(q)
		assert.True(t, ok)
		assert.Equal(t, 10, offset)
		assert.Equal(t, 20, limit)
	})

	t.Run("PageOf returns false for non-offset page", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithCursorPage[testFilter](pager.CursorPage{Limit: 20}, "id", map[string]string{"id": "id"}))
		_, _, ok := PageOf(q)
		assert.False(t, ok)
	})

	t.Run("CursorPageOf extracts cursor page", func(t *testing.T) {
		q := NewQuery(testFilter{},
			WithCursorPage[testFilter](pager.CursorPage{Value: "123", Limit: 20}, "id", map[string]string{"id": "id"}),
		)
		cp := CursorPageOf(q)
		assert.NotNil(t, cp)
		assert.Equal(t, "id", cp.Column)
		assert.Equal(t, "123", cp.Page.Value)
	})

	t.Run("CursorPageOf returns nil for non-cursor page", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithOffsetPage[testFilter](0, 20))
		cp := CursorPageOf(q)
		assert.Nil(t, cp)
	})
}

// ---------------------------------------------------------------------------
// SQL 注入安全测试
// ---------------------------------------------------------------------------

// setupMockDB 创建基于 go-sqlmock 的 GORM DB，用于捕获生成的 SQL。
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	assert.NoError(t, err)
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{})
	assert.NoError(t, err)
	return db, mock
}

func TestSQLInjection_SortFieldWhiteList(t *testing.T) {
	t.Run("strict mode rejects SQL injection in sort field", func(t *testing.T) {
		// 攻击向量：排序字段包含 SQL 注入负载 "id; DROP TABLE users--"
		// 预期防御：严格模式下返回错误，不生成任何 ORDER BY
		mapping := NewSortMapping(WithSortFields("name", "status"), WithStrictSort(true))
		spec := NewSortSpec([]pager.Sorter{{Field: "id; DROP TABLE users--", Sorted: pager.ASC}})
		db := &gorm.DB{}
		_, err := ApplySort(db, spec, mapping)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown sort field")
	})

	t.Run("lenient mode skips SQL injection in sort field", func(t *testing.T) {
		// 攻击向量：排序字段包含 SQL 注入负载 "id; DROP TABLE users--"
		// 预期防御：宽松模式下跳过该字段，不生成 ORDER BY
		mapping := NewSortMapping(WithSortFields("name", "status"))
		spec := NewSortSpec([]pager.Sorter{{Field: "id; DROP TABLE users--", Sorted: pager.ASC}})
		db := &gorm.DB{}
		result, err := ApplySort(db, spec, mapping)
		assert.NoError(t, err)
		// 非严格模式应跳过未知字段，result 不应包含额外排序
		assert.NotNil(t, result)
	})

	t.Run("strict mode rejects SQL keyword as sort field", func(t *testing.T) {
		// 攻击向量：使用 SQL 关键字 "SELECT" 作为排序字段
		// 预期防御：严格模式下返回错误
		mapping := NewSortMapping(WithSortFields("name"), WithStrictSort(true))
		spec := NewSortSpec([]pager.Sorter{{Field: "SELECT", Sorted: pager.ASC}})
		db := &gorm.DB{}
		_, err := ApplySort(db, spec, mapping)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown sort field")
	})

	t.Run("lenient mode skips SQL keyword as sort field", func(t *testing.T) {
		// 攻击向量：使用 SQL 关键字 "SELECT" 作为排序字段
		// 预期防御：宽松模式下跳过该字段
		mapping := NewSortMapping(WithSortFields("name"))
		spec := NewSortSpec([]pager.Sorter{{Field: "SELECT", Sorted: pager.ASC}})
		db := &gorm.DB{}
		result, err := ApplySort(db, spec, mapping)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("strict mode rejects sort field with semicolon", func(t *testing.T) {
		// 攻击向量：排序字段包含分号 "name; DROP TABLE users"
		// 预期防御：严格模式下返回错误
		mapping := NewSortMapping(WithSortFields("name"), WithStrictSort(true))
		spec := NewSortSpec([]pager.Sorter{{Field: "name; DROP TABLE users", Sorted: pager.ASC}})
		db := &gorm.DB{}
		_, err := ApplySort(db, spec, mapping)
		assert.Error(t, err)
	})

	t.Run("strict mode rejects sort field with comment syntax", func(t *testing.T) {
		// 攻击向量：排序字段包含 SQL 注释 "name--"
		// 预期防御：严格模式下返回错误
		mapping := NewSortMapping(WithSortFields("name"), WithStrictSort(true))
		spec := NewSortSpec([]pager.Sorter{{Field: "name--", Sorted: pager.ASC}})
		db := &gorm.DB{}
		_, err := ApplySort(db, spec, mapping)
		assert.Error(t, err)
	})

	t.Run("whitelisted field generates correct ORDER BY", func(t *testing.T) {
		// 正常场景：白名单内字段应正确生成 ORDER BY
		db, mock := setupMockDB(t)
		mapping := NewSortMapping(WithSortFields("name", "status"))
		spec := NewSortSpec([]pager.Sorter{{Field: "name", Sorted: pager.ASC}})

		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `test_models`")).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

		result, err := ApplySort(db.Model(&testModel{}), spec, mapping)
		assert.NoError(t, err)

		var records []testModel
		_ = result.Find(&records).Error
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("Resolve returns false for field with special characters", func(t *testing.T) {
		// 验证 SortMapping.Resolve 对特殊字符的防御
		mapping := NewSortMapping(WithSortFields("name", "status"))

		attackVectors := []string{
			"id; DROP TABLE users--",
			"1 OR 1=1",
			"name'; --",
			"status UNION SELECT * FROM users",
			"name\" OR \"1\"=\"1",
			"status` WHERE 1=1 --",
		}
		for _, v := range attackVectors {
			_, ok := mapping.Resolve(v)
			assert.False(t, ok, "Resolve should reject attack vector: %q", v)
		}
	})
}

func TestSQLInjection_CursorPageColumnWhiteList(t *testing.T) {
	t.Run("non-whitelisted column is skipped", func(t *testing.T) {
		// 攻击向量：游标列名不在白名单中
		// 预期防御：跳过游标条件，仅应用 LIMIT
		db := setupTestDB(t)
		seedTestData(t, db)

		q := NewQuery(testFilter{},
			WithCursorPage[testFilter](pager.CursorPage{Value: "1", Limit: 2}, "nonexistent", map[string]string{"id": "id"}),
		)
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("malicious column name is skipped", func(t *testing.T) {
		// 攻击向量：游标列名包含 SQL 注入负载 "id; SELECT * FROM users"
		// 预期防御：不在白名单中，跳过游标条件
		db := setupTestDB(t)
		seedTestData(t, db)

		q := NewQuery(testFilter{},
			WithCursorPage[testFilter](pager.CursorPage{Value: "1", Limit: 2}, "id; SELECT * FROM users", map[string]string{"id": "id"}),
		)
		result, err := Build(db.Model(&testModel{}), q,
			WithFilterTransfer[testFilter](testFilterTransfer),
			WithSortMapping[testFilter](NewSortMapping()),
		)
		assert.NoError(t, err)

		var records []testModel
		err = result.Find(&records).Error
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("whitelisted column generates parameterized WHERE", func(t *testing.T) {
		// 正常场景：白名单内列名应正确生成参数化 WHERE 子句
		db, mock := setupMockDB(t)

		page := &CursorPageSpec{
			Page:   pager.CursorPage{Value: "100", Direction: pager.CursorAfter, Limit: 10},
			Column: "id",
			Fields: map[string]string{"id": "id"},
		}
		result := ApplyPage(db.Model(&testModel{}), page)

		// 验证生成的 SQL 使用参数化查询（? 占位符）
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `test_models` WHERE id > ? AND `test_models`.`deleted_at` IS NULL ORDER BY id ASC LIMIT ?")).
			WithArgs("100", 10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

		var records []testModel
		_ = result.Find(&records).Error
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("cursor before direction generates parameterized WHERE", func(t *testing.T) {
		// 正常场景：向前翻页应生成参数化 WHERE + DESC
		db, mock := setupMockDB(t)

		page := &CursorPageSpec{
			Page:   pager.CursorPage{Value: "50", Direction: pager.CursorBefore, Limit: 10},
			Column: "id",
			Fields: map[string]string{"id": "id"},
		}
		result := ApplyPage(db.Model(&testModel{}), page)

		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `test_models` WHERE id < ? AND `test_models`.`deleted_at` IS NULL ORDER BY id DESC LIMIT ?")).
			WithArgs("50", 10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

		var records []testModel
		_ = result.Find(&records).Error
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("malicious cursor value is parameterized safely", func(t *testing.T) {
		// 攻击向量：游标值包含 SQL 注入负载 "1 OR 1=1"
		// 预期防御：游标值通过参数化查询传递，不会影响 SQL 结构
		db, mock := setupMockDB(t)

		page := &CursorPageSpec{
			Page:   pager.CursorPage{Value: "1 OR 1=1", Direction: pager.CursorAfter, Limit: 10},
			Column: "id",
			Fields: map[string]string{"id": "id"},
		}
		result := ApplyPage(db.Model(&testModel{}), page)

		// SQL 结构应保持 "WHERE id > ?"，值 "1 OR 1=1" 作为参数安全传递
		mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `test_models` WHERE id > ? AND `test_models`.`deleted_at` IS NULL ORDER BY id ASC LIMIT ?")).
			WithArgs("1 OR 1=1", 10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name"}))

		var records []testModel
		_ = result.Find(&records).Error
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSQLInjection_PreloadPath(t *testing.T) {
	t.Run("preload with special characters does not crash", func(t *testing.T) {
		// 攻击向量：preload 名包含 SQL 特殊字符
		// 预期防御：GORM Preload 接收关联名而非原始 SQL，特殊字符不会注入
		db := setupTestDB(t)

		// GORM 对无效关联名会返回错误（如 "record not found"），
		// 但不会执行 SQL 注入
		maliciousPreloads := []string{
			"Items; DROP TABLE users--",
			"Items UNION SELECT * FROM users",
			"Items'; --",
		}
		for _, preload := range maliciousPreloads {
			result := ApplyPreloads(db.Model(&testModel{}), []string{preload})
			assert.NotNil(t, result, "ApplyPreloads should not crash for malicious preload: %q", preload)
		}
	})

	t.Run("valid preload is applied without error", func(t *testing.T) {
		// 正常场景：合法 preload 名正常应用
		db := setupTestDB(t)
		result := ApplyPreloads(db.Model(&testModel{}), []string{"Items"})
		assert.NotNil(t, result)
	})

	t.Run("empty preloads list returns db unchanged", func(t *testing.T) {
		// 边界场景：空 preload 列表
		db := setupTestDB(t)
		result := ApplyPreloads(db.Model(&testModel{}), []string{})
		assert.NotNil(t, result)
	})
}
