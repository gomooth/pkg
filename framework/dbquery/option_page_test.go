package dbquery

import (
	"testing"

	"github.com/gomooth/pkg/framework/pager"
	"github.com/stretchr/testify/assert"
)

// ==================== WithSkipPage ====================

func TestWithSkipPage(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	// WithSkipPage should skip pagination in Build
	q := NewQuery(testFilter{}, WithOffsetPage[testFilter](0, 2))
	result, err := Build(db.Model(&testModel{}), q,
		WithFilterTransfer[testFilter](testFilterTransfer),
		WithSortMapping[testFilter](NewSortMapping()),
		WithSkipPage[testFilter](),
	)
	assert.NoError(t, err)

	var records []testModel
	err = result.Find(&records).Error
	assert.NoError(t, err)
	// Skip page means no LIMIT applied, so all 5 records returned
	assert.Len(t, records, 5)
}

// ==================== WithSortSpec ====================

func TestWithSortSpec(t *testing.T) {
	db := setupTestDB(t)
	seedTestData(t, db)

	spec := NewSortSpec([]pager.Sorter{{Field: "name", Sorted: pager.ASC}})
	q := NewQuery(testFilter{}, WithSortSpec[testFilter](spec))
	mapping := NewSortMapping(WithSortFields("name"))
	result, err := Build(db.Model(&testModel{}), q,
		WithFilterTransfer[testFilter](testFilterTransfer),
		WithSortMapping[testFilter](mapping),
	)
	assert.NoError(t, err)

	var records []testModel
	err = result.Find(&records).Error
	assert.NoError(t, err)
	assert.Equal(t, "Alice", records[0].Name)
}

// ==================== WithOffsetPageMax ====================

func TestWithOffsetPageMax(t *testing.T) {
	t.Run("limit capped by maxLimit", func(t *testing.T) {
		opt := WithOffsetPageMax[testFilter](0, 1000, 3)
		o := &queryOption[testFilter]{}
		opt(o)
		assert.Equal(t, OffsetPage{Offset: 0, Limit: 3}, o.page)
	})

	t.Run("zero limit uses default", func(t *testing.T) {
		opt := WithOffsetPageMax[testFilter](0, 0, 100)
		o := &queryOption[testFilter]{}
		opt(o)
		assert.Equal(t, OffsetPage{Offset: 0, Limit: pager.DefaultPageSize}, o.page)
	})

	t.Run("zero maxLimit uses 100 default", func(t *testing.T) {
		opt := WithOffsetPageMax[testFilter](0, 200, 0)
		o := &queryOption[testFilter]{}
		opt(o)
		assert.Equal(t, OffsetPage{Offset: 0, Limit: 100}, o.page)
	})
}

// ==================== CursorPageSpec.String ====================

func TestCursorPageSpec_String(t *testing.T) {
	cp := &CursorPageSpec{
		Page:   pager.CursorPage{Value: "100", Direction: pager.CursorAfter, Limit: 20},
		Column: "id",
		Fields: map[string]string{"id": "id"},
	}
	s := cp.String()
	assert.Contains(t, s, `"value":"100"`)
	assert.Contains(t, s, `"direction":0`) // CursorAfter = 0
	assert.Contains(t, s, `"limit":20`)
	assert.Contains(t, s, `"column":"id"`)

	// Also test CursorBefore
	cp2 := &CursorPageSpec{
		Page:   pager.CursorPage{Value: "50", Direction: pager.CursorBefore, Limit: 10},
		Column: "id",
		Fields: map[string]string{"id": "id"},
	}
	s2 := cp2.String()
	assert.Contains(t, s2, `"direction":1`) // CursorBefore = 1
}

// ==================== PaginateValues ====================

func TestPaginateValues(t *testing.T) {
	t.Run("nil query returns not paginated", func(t *testing.T) {
		start, limit, paginated := PaginateValues[testFilter](nil)
		assert.Equal(t, 0, start)
		assert.Equal(t, 0, limit)
		assert.False(t, paginated)
	})

	t.Run("query without page returns not paginated", func(t *testing.T) {
		q := NewQuery(testFilter{})
		start, limit, paginated := PaginateValues(q)
		assert.Equal(t, 0, start)
		assert.Equal(t, 0, limit)
		assert.False(t, paginated)
	})

	t.Run("offset page returns correct values", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithOffsetPage[testFilter](10, 20))
		start, limit, paginated := PaginateValues(q)
		assert.Equal(t, 10, start)
		assert.Equal(t, 20, limit)
		assert.True(t, paginated)
	})

	t.Run("cursor page returns correct values", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithCursorPage[testFilter](pager.CursorPage{Limit: 15}, "id", map[string]string{"id": "id"}))
		start, limit, paginated := PaginateValues(q)
		assert.Equal(t, 0, start)
		assert.Equal(t, 15, limit)
		assert.True(t, paginated)
	})
}

// ==================== CacheKeyPart ====================

func TestCacheKeyPart(t *testing.T) {
	t.Run("nil query returns empty", func(t *testing.T) {
		assert.Equal(t, "", CacheKeyPart[testFilter](nil))
	})

	t.Run("query without page returns empty", func(t *testing.T) {
		q := NewQuery(testFilter{})
		assert.Equal(t, "", CacheKeyPart(q))
	})

	t.Run("query with page returns page string", func(t *testing.T) {
		q := NewQuery(testFilter{}, WithOffsetPage[testFilter](10, 20))
		assert.NotEmpty(t, CacheKeyPart(q))
	})
}

// ==================== FormatPaginateKey ====================

func TestFormatPaginateKey(t *testing.T) {
	key := FormatPaginateKey("users", 0, 20, "abc123")
	assert.Equal(t, "users:paginate:0,20:abc123", key)
}

// ==================== FormatListKey ====================

func TestFormatListKey(t *testing.T) {
	key := FormatListKey("users", "abc123")
	assert.Equal(t, "users:list:abc123", key)
}

// ==================== Filter nil branch ====================

func TestFilter_NilFilter(t *testing.T) {
	q := &Query[testFilter]{filter: nil}
	f := q.Filter()
	assert.NotNil(t, f)
}

// ==================== Sorters nil branch ====================

func TestSortSpec_NilSorters(t *testing.T) {
	s := &sortSpec{sorters: nil}
	assert.Equal(t, []pager.Sorter{}, s.Sorters())
}

// ==================== PageOf nil branches ====================

func TestPageOf_NilQuery(t *testing.T) {
	offset, limit, ok := PageOf[testFilter](nil)
	assert.Equal(t, 0, offset)
	assert.Equal(t, pager.DefaultPageSize, limit)
	assert.False(t, ok)
}

func TestPageOf_NilPage(t *testing.T) {
	q := NewQuery(testFilter{})
	offset, limit, ok := PageOf(q)
	assert.Equal(t, 0, offset)
	assert.Equal(t, pager.DefaultPageSize, limit)
	assert.False(t, ok)
}

// ==================== CursorPageOf nil branches ====================

func TestCursorPageOf_NilQuery(t *testing.T) {
	cp := CursorPageOf[testFilter](nil)
	assert.Nil(t, cp)
}

func TestCursorPageOf_NilPage(t *testing.T) {
	q := NewQuery(testFilter{})
	cp := CursorPageOf(q)
	assert.Nil(t, cp)
}
