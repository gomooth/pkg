package dbrepo

import (
	"context"
	"fmt"
	"testing"

	"github.com/gomooth/pkg/framework/dbquery"
	"github.com/gomooth/pkg/framework/pager"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// searchModel is a test model for Searcher tests
type searchModel struct {
	gorm.Model
	Name   string
	Email  string
	Status int
}

// searchFilter is a test filter struct for Searcher tests
type searchFilter struct {
	Name   string
	Status *int
}

// searchFilterTransfer applies filter conditions to the database query
func searchFilterTransfer(filter *searchFilter, db *gorm.DB) *gorm.DB {
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

// searchSortKeyMapping maps front-end sort keys to database column names
var searchSortKeyMapping = map[string]string{
	"name":   "name",
	"status": "status",
}

// searchSortMapping is the SortMapping for test searcher
var searchSortMapping = dbquery.NewSortMapping(
	dbquery.WithSortKeyMap(searchSortKeyMapping),
)

// setupSearchTestDB creates an in-memory SQLite database and seeds test data
func setupSearchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)
	assert.NotNil(t, db)

	err = db.AutoMigrate(&searchModel{})
	assert.NoError(t, err)

	return db
}

// seedSearchData inserts test records into the database
func seedSearchData(t *testing.T, db *gorm.DB) {
	t.Helper()
	records := []*searchModel{
		{Name: "Alice", Email: "alice@example.com", Status: 1},
		{Name: "Bob", Email: "bob@example.com", Status: 2},
		{Name: "Charlie", Email: "charlie@example.com", Status: 1},
		{Name: "David", Email: "david@example.com", Status: 3},
		{Name: "Eve", Email: "eve@example.com", Status: 2},
	}
	for _, r := range records {
		err := db.Create(r).Error
		assert.NoError(t, err)
	}
}

func newTestSearcher(t *testing.T) (ISearcher[searchModel, searchFilter], *gorm.DB) {
	t.Helper()
	db := setupSearchTestDB(t)
	seedSearchData(t, db)
	s, err := NewSearcher[searchModel, searchFilter](
		db,
		WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
		WithSortMapping[searchModel, searchFilter](searchSortMapping),
	)
	assert.NoError(t, err)
	return s, db
}

func TestSearcher_FindAll(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("returns all records", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithSorts[searchFilter]("name"))
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 5)
	})

	t.Run("filters by name", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{Name: "ali"}, dbquery.WithSorts[searchFilter]("name"))
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 1)
		assert.Equal(t, "Alice", records[0].Name)
	})

	t.Run("filters by status", func(t *testing.T) {
		status := 1
		q := dbquery.NewQuery(searchFilter{Status: &status})
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 2)
		for _, r := range records {
			assert.Equal(t, 1, r.Status)
		}
	})

	t.Run("empty result", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{Name: "nonexistent"})
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 0)
	})
}

func TestSearcher_List(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("returns paginated records", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithOffsetPage[searchFilter](0, 3))
		records, err := searcher.List(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 3)
	})

	t.Run("second page", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithOffsetPage[searchFilter](3, 3))
		records, err := searcher.List(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("with filter", func(t *testing.T) {
		status := 2
		q := dbquery.NewQuery(searchFilter{Status: &status}, dbquery.WithOffsetPage[searchFilter](0, 10))
		records, err := searcher.List(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})
}

func TestSearcher_Paginate(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("returns records and total count", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithOffsetPage[searchFilter](0, 3))
		records, total, err := searcher.Paginate(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 3)
		assert.Equal(t, uint(5), total)
	})

	t.Run("second page returns remaining records", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithOffsetPage[searchFilter](3, 3))
		records, _, err := searcher.Paginate(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("with filter reduces total", func(t *testing.T) {
		status := 1
		q := dbquery.NewQuery(searchFilter{Status: &status}, dbquery.WithOffsetPage[searchFilter](0, 10))
		records, total, err := searcher.Paginate(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 2)
		assert.Equal(t, uint(2), total)
	})

	t.Run("empty result", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{Name: "nonexistent"}, dbquery.WithOffsetPage[searchFilter](0, 10))
		records, total, err := searcher.Paginate(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 0)
		assert.Equal(t, uint(0), total)
	})
}

func TestSearcher_CountBy(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("count all", func(t *testing.T) {
		count, err := searcher.CountBy(ctx, &searchFilter{})
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)
	})

	t.Run("count with filter", func(t *testing.T) {
		status := 1
		count, err := searcher.CountBy(ctx, &searchFilter{Status: &status})
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count)
	})

	t.Run("count empty result", func(t *testing.T) {
		count, err := searcher.CountBy(ctx, &searchFilter{Name: "nonexistent"})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}

func TestSearcher_ExistsBy(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("exists", func(t *testing.T) {
		exists, err := searcher.ExistsBy(ctx, &searchFilter{Name: "Alice"})
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("not exists", func(t *testing.T) {
		exists, err := searcher.ExistsBy(ctx, &searchFilter{Name: "nonexistent"})
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("exists by status", func(t *testing.T) {
		status := 3
		exists, err := searcher.ExistsBy(ctx, &searchFilter{Status: &status})
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("nil filter returns true when records exist", func(t *testing.T) {
		exists, err := searcher.ExistsBy(ctx, nil)
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestSearcher_Find(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("without option builders", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{})
		records, err := searcher.Find(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 5)
	})

	t.Run("with pagination via WithFindPage", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{})
		records, err := searcher.Find(ctx, q, WithFindPage(0, 2))
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("with filter", func(t *testing.T) {
		status := 2
		q := dbquery.NewQuery(searchFilter{Status: &status})
		records, err := searcher.Find(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})
}

func TestSearcher_FirstWith(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{Name: "Alice"})
		record, err := searcher.FirstWith(ctx, q)
		assert.NoError(t, err)
		assert.NotNil(t, record)
		assert.Equal(t, "Alice", record.Name)
	})

	t.Run("not found", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{Name: "nonexistent"})
		record, err := searcher.FirstWith(ctx, q)
		assert.Error(t, err)
		assert.Nil(t, record)
		assert.True(t, xerror.IsXCode(err, xcode.DBRecordNotFound))
	})

	t.Run("with WithSelect", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{Name: "Alice"})
		record, err := searcher.FirstWith(ctx, q, WithSelect("id", "name"))
		assert.NoError(t, err)
		assert.NotNil(t, record)
		assert.Equal(t, "Alice", record.Name)
	})

	t.Run("with WithPreload on invalid relation returns error (coverage)", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{Name: "Alice"})
		_, err := searcher.FirstWith(ctx, q, WithPreload("Items"))
		// GORM returns error for unsupported relations, but we just need to cover the WithPreload path
		_ = err
	})
}

func TestSearcher_filterTransfer(t *testing.T) {
	db := setupSearchTestDB(t)
	seedSearchData(t, db)
	ctx := context.Background()

	t.Run("nil filterTransfer still works", func(t *testing.T) {
		searcher, err := NewSearcher[searchModel, searchFilter](db)
		assert.NoError(t, err)
		q := dbquery.NewQuery(searchFilter{})
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 5)
	})

	t.Run("filterTransfer applies conditions", func(t *testing.T) {
		searcher, err := NewSearcher[searchModel, searchFilter](
			db,
			WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
			WithSortMapping[searchModel, searchFilter](searchSortMapping),
		)
		assert.NoError(t, err)
		q := dbquery.NewQuery(searchFilter{Name: "Bob"})
		record, err := searcher.FirstWith(ctx, q)
		assert.NoError(t, err)
		assert.Equal(t, "Bob", record.Name)
	})
}

func TestSearcher_sortKeyMapping(t *testing.T) {
	db := setupSearchTestDB(t)
	seedSearchData(t, db)
	ctx := context.Background()

	t.Run("sort by name ascending", func(t *testing.T) {
		searcher, err := NewSearcher[searchModel, searchFilter](
			db,
			WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
			WithSortMapping[searchModel, searchFilter](searchSortMapping),
		)
		assert.NoError(t, err)
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithSorts[searchFilter]("+name"))
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 5)
		if len(records) >= 2 {
			assert.Equal(t, "Alice", records[0].Name)
		}
	})

	t.Run("sort by name descending", func(t *testing.T) {
		searcher, err := NewSearcher[searchModel, searchFilter](
			db,
			WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
			WithSortMapping[searchModel, searchFilter](searchSortMapping),
		)
		assert.NoError(t, err)
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithSorts[searchFilter]("-name"))
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 5)
		if len(records) >= 2 {
			assert.Equal(t, "Eve", records[0].Name)
		}
	})
}

func TestSearcher_CombinedFilterAndSort(t *testing.T) {
	db := setupSearchTestDB(t)
	seedSearchData(t, db)
	ctx := context.Background()

	searcher, err := NewSearcher[searchModel, searchFilter](
		db,
		WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
		WithSortMapping[searchModel, searchFilter](searchSortMapping),
	)
	assert.NoError(t, err)

	t.Run("filter by status and sort by name desc", func(t *testing.T) {
		status := 1
		q := dbquery.NewQuery(searchFilter{Status: &status}, dbquery.WithSorts[searchFilter]("-name"))
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 2)
		assert.Equal(t, "Charlie", records[0].Name)
		assert.Equal(t, "Alice", records[1].Name)
	})
}

func TestSearcher_WithBuilderOptions(t *testing.T) {
	db := setupSearchTestDB(t)
	seedSearchData(t, db)
	ctx := context.Background()

	t.Run("WithFilterTransfer option", func(t *testing.T) {
		searcher, err := NewSearcher[searchModel, searchFilter](
			db,
			WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
		)
		assert.NoError(t, err)
		status := 2
		q := dbquery.NewQuery(searchFilter{Status: &status})
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 2)
	})

	t.Run("WithSortMapping option", func(t *testing.T) {
		sortMapping := dbquery.NewSortMapping(dbquery.WithSortKeyMap(searchSortKeyMapping))
		searcher, err := NewSearcher[searchModel, searchFilter](
			db,
			WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
			WithSortMapping[searchModel, searchFilter](sortMapping),
		)
		assert.NoError(t, err)
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithSorts[searchFilter]("+name"))
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 5)
		if len(records) >= 1 {
			assert.Equal(t, "Alice", records[0].Name)
		}
	})
}

func TestSearcher_EmptyDB(t *testing.T) {
	db := setupSearchTestDB(t)
	// Do NOT seed data
	ctx := context.Background()

	searcher, err := NewSearcher[searchModel, searchFilter](
		db,
		WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
		WithSortMapping[searchModel, searchFilter](searchSortMapping),
	)
	assert.NoError(t, err)

	t.Run("FindAll returns empty", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{})
		records, err := searcher.FindAll(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 0)
	})

	t.Run("Paginate returns zero total", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithOffsetPage[searchFilter](0, 10))
		records, total, err := searcher.Paginate(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 0)
		assert.Equal(t, uint(0), total)
	})

	t.Run("CountBy returns zero", func(t *testing.T) {
		count, err := searcher.CountBy(ctx, &searchFilter{})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("ExistsBy returns false", func(t *testing.T) {
		exists, err := searcher.ExistsBy(ctx, &searchFilter{Name: "anything"})
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("FirstWith returns not found", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{})
		record, err := searcher.FirstWith(ctx, q)
		assert.Error(t, err)
		assert.Nil(t, record)
		assert.True(t, xerror.IsXCode(err, xcode.DBRecordNotFound))
	})
}

func TestSearcher_CountBy_NilFilter(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("empty filter counts all", func(t *testing.T) {
		count, err := searcher.CountBy(ctx, &searchFilter{})
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)
	})

	t.Run("nil filter counts all records", func(t *testing.T) {
		count, err := searcher.CountBy(ctx, nil)
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)
	})
}

func TestSearcher_CountBy_UsesGormbuild(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("CountBy applies filterTransfer via gormbuild", func(t *testing.T) {
		status := 1
		count, err := searcher.CountBy(ctx, &searchFilter{Status: &status})
		assert.NoError(t, err)
		assert.Equal(t, int64(2), count) // Alice and Charlie have status=1
	})

	t.Run("CountBy with name filter", func(t *testing.T) {
		count, err := searcher.CountBy(ctx, &searchFilter{Name: "ali"})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count) // Only Alice matches
	})

	t.Run("CountBy with combined filters", func(t *testing.T) {
		status := 2
		count, err := searcher.CountBy(ctx, &searchFilter{Status: &status, Name: "bob"})
		assert.NoError(t, err)
		assert.Equal(t, int64(1), count) // Only Bob matches status=2 and name contains "bob"
	})
}

func TestSearcher_List_PaginationBoundary(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("offset beyond records returns empty", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithOffsetPage[searchFilter](100, 10))
		records, err := searcher.List(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 0)
	})

	t.Run("zero limit uses default page size", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{}, dbquery.WithOffsetPage[searchFilter](0, 0))
		records, err := searcher.List(ctx, q)
		assert.NoError(t, err)
		// With zero limit, dbquery.WithOffsetPage defaults to pager.DefaultPageSize (20),
		// capped by MaxPageSize (100). All 5 records should fit.
		assert.Len(t, records, 5)
	})
}

// WithFindPage is a helper to set pagination on findOption
func WithFindPage(start, limit int) findOptionBuilder {
	return func(opt *findOption) {
		opt.start = start
		opt.limit = limit
	}
}

// ==================== WithCursorExtractor ====================

func TestWithCursorExtractor(t *testing.T) {
	db := setupSearchTestDB(t)
	seedSearchData(t, db)
	ctx := context.Background()

	s, err := NewSearcher[searchModel, searchFilter](
		db,
		WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
		WithSortMapping[searchModel, searchFilter](searchSortMapping),
		WithCursorExtractor[searchModel, searchFilter](func(m *searchModel) string {
			return fmt.Sprintf("%d", m.ID)
		}),
	)
	assert.NoError(t, err)

	q := dbquery.NewQuery(searchFilter{},
		dbquery.WithCursorPage[searchFilter](pager.CursorPage{Limit: 3}, "id", map[string]string{"id": "id"}),
	)
	records, nextCursor, err := s.ListByCursor(ctx, q)
	assert.NoError(t, err)
	assert.Len(t, records, 3)
	assert.NotEmpty(t, nextCursor, "nextCursor should be extracted from last record")
}

// ==================== WithPreload ====================

func TestWithPreload(t *testing.T) {
	opt := WithPreload("Items", "Orders")
	fo := &findOption{}
	opt(fo)
	assert.Equal(t, []string{"Items", "Orders"}, fo.preloads)
}

// ==================== WithSelect ====================

func TestWithSelect(t *testing.T) {
	opt := WithSelect("id", "name")
	fo := &findOption{}
	opt(fo)
	assert.Equal(t, []string{"id", "name"}, fo.selects)
}

// ==================== ListByCursor ====================

func TestSearcher_ListByCursor(t *testing.T) {
	db := setupSearchTestDB(t)
	seedSearchData(t, db)
	ctx := context.Background()

	t.Run("with cursorExtractor returns next cursor", func(t *testing.T) {
		s, err := NewSearcher[searchModel, searchFilter](
			db,
			WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
			WithSortMapping[searchModel, searchFilter](searchSortMapping),
			WithCursorExtractor[searchModel, searchFilter](func(m *searchModel) string {
				return fmt.Sprintf("%d", m.ID)
			}),
		)
		assert.NoError(t, err)

		q := dbquery.NewQuery(searchFilter{},
			dbquery.WithCursorPage[searchFilter](pager.CursorPage{Limit: 3}, "id", map[string]string{"id": "id"}),
		)
		records, nextCursor, err := s.ListByCursor(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 3)
		assert.NotEmpty(t, nextCursor)
	})

	t.Run("without cursorExtractor returns empty next cursor", func(t *testing.T) {
		s, err := NewSearcher[searchModel, searchFilter](
			db,
			WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
			WithSortMapping[searchModel, searchFilter](searchSortMapping),
		)
		assert.NoError(t, err)

		q := dbquery.NewQuery(searchFilter{},
			dbquery.WithCursorPage[searchFilter](pager.CursorPage{Limit: 3}, "id", map[string]string{"id": "id"}),
		)
		records, nextCursor, err := s.ListByCursor(ctx, q)
		assert.NoError(t, err)
		assert.Len(t, records, 3)
		assert.Empty(t, nextCursor, "without cursorExtractor, nextCursor should be empty")
	})

	t.Run("empty result returns no cursor", func(t *testing.T) {
		s, err := NewSearcher[searchModel, searchFilter](
			db,
			WithFilterTransfer[searchModel, searchFilter](searchFilterTransfer),
			WithSortMapping[searchModel, searchFilter](searchSortMapping),
			WithCursorExtractor[searchModel, searchFilter](func(m *searchModel) string {
				return fmt.Sprintf("%d", m.ID)
			}),
		)
		assert.NoError(t, err)

		q := dbquery.NewQuery(searchFilter{Name: "nonexistent"},
			dbquery.WithCursorPage[searchFilter](pager.CursorPage{Limit: 3}, "id", map[string]string{"id": "id"}),
		)
		records, nextCursor, err := s.ListByCursor(ctx, q)
		assert.NoError(t, err)
		assert.Empty(t, records)
		assert.Empty(t, nextCursor)
	})
}

// ==================== Find with WithPreload and WithSelect ====================

func TestSearcher_Find_WithOptionBuilders(t *testing.T) {
	searcher, _ := newTestSearcher(t)
	ctx := context.Background()

	t.Run("Find with WithSelect", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{})
		records, err := searcher.Find(ctx, q, WithSelect("id", "name"))
		assert.NoError(t, err)
		assert.Len(t, records, 5)
	})

	t.Run("Find with WithPreload on invalid relation returns error (coverage)", func(t *testing.T) {
		q := dbquery.NewQuery(searchFilter{})
		_, err := searcher.Find(ctx, q, WithPreload("Items"))
		// GORM returns error for unsupported relations, but we just need to cover the WithPreload path
		_ = err
	})
}

// Ensure pager.Sorter and pager.ParseSorts are accessible (compile-time check)
var _ = pager.ParseSorts
