package dbrepo

import (
	"context"
	"errors"
	"time"

	"github.com/gomooth/pkg/framework/dbquery"
	"github.com/gomooth/pkg/framework/pager"
	"github.com/gomooth/xerror"
	"github.com/gomooth/xerror/xcode"
	"gorm.io/gorm"
)

// IListSearcher 列表查询，返回实体
type IListSearcher[M, F any] interface {
	// FindAll 查询所有记录
	FindAll(ctx context.Context, q dbquery.IQuery[F]) ([]*M, error)
	// List 分页查询记录（不返回总数）
	List(ctx context.Context, q dbquery.IQuery[F]) ([]*M, error)
	// Paginate 分页查询记录（返回总数）
	Paginate(ctx context.Context, q dbquery.IQuery[F]) ([]*M, uint, error)
	// ListByCursor 游标分页查询记录（高性能，大数据量场景替代 offset 分页）
	// 游标参数和列白名单在 IQuery.Page 的 CursorPageSpec 中配置
	// 返回结果集和下一页游标值；无更多数据时 nextCursor 为空字符串
	ListByCursor(ctx context.Context, q dbquery.IQuery[F]) ([]*M, pager.Cursor, error)
	// Find 通用查询方法
	Find(ctx context.Context, q dbquery.IQuery[F], optBuilders ...findOptionBuilder) ([]*M, error)
	// FirstWith 带选项查询单条记录
	FirstWith(ctx context.Context, q dbquery.IQuery[F], optBuilders ...findOptionBuilder) (*M, error)
}

// IAggSearcher 聚合查询，返回计数或存在性
type IAggSearcher[F any] interface {
	// CountBy 统计记录数量
	CountBy(ctx context.Context, filter *F) (int64, error)
	// ExistsBy 判断记录是否存在
	ExistsBy(ctx context.Context, filter *F) (bool, error)
}

// ISearcher 搜索器组合接口
type ISearcher[M any, F any] interface {
	IListSearcher[M, F]
	IAggSearcher[F]
}

// searcher 查询构建器，提供通用的查询方法
type searcher[M any, F any] struct {
	db              *gorm.DB
	filterTransfer  func(filter *F, db *gorm.DB) *gorm.DB
	sortMapping     *dbquery.SortMapping
	cursorExtractor func(*M) string
}

// NewSearcher 创建查询构建器
// db 不能为 nil，否则返回错误
func NewSearcher[M any, F any](db *gorm.DB, opts ...SearcherOption[M, F]) (ISearcher[M, F], error) {
	if db == nil {
		return nil, xerror.New("dbrepo: NewSearcher called with nil *gorm.DB")
	}

	s := &searcher[M, F]{
		db:          db,
		sortMapping: dbquery.NewSortMapping(),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// buildQuery 构建 GORM 查询
func (q *searcher[M, F]) buildQuery(ctx context.Context, query dbquery.IQuery[F], extraOpts ...dbquery.BuildOption[F]) (*gorm.DB, error) {
	db := q.db.WithContext(ctx).Model(new(M))
	opts := []dbquery.BuildOption[F]{
		dbquery.WithFilterTransfer[F](q.filterTransfer),
		dbquery.WithSortMapping[F](q.sortMapping),
	}
	opts = append(opts, extraOpts...)
	return dbquery.Build(db, query, opts...)
}

// FindAll 查询所有记录
func (q *searcher[M, F]) FindAll(ctx context.Context, query dbquery.IQuery[F]) (records []*M, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "searcher", "find_all", time.Since(start), err)
	}()

	db, err := q.buildQuery(ctx, query)
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBRequestParamError)
	}

	if err := db.Find(&records).Error; err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return records, nil
}

// List 分页查询记录（不返回总数）
func (q *searcher[M, F]) List(ctx context.Context, query dbquery.IQuery[F]) (records []*M, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "searcher", "list", time.Since(start), err)
	}()

	db, err := q.buildQuery(ctx, query)
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBRequestParamError)
	}

	if err := db.Find(&records).Error; err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return records, nil
}

// Paginate 分页查询记录（返回总数）
func (q *searcher[M, F]) Paginate(ctx context.Context, query dbquery.IQuery[F]) (records []*M, total uint, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "searcher", "paginate", time.Since(start), err)
	}()

	// Count 查询跳过分页，避免 offset/limit 影响总数
	countDB, err := q.buildQuery(ctx, query, dbquery.WithSkipPage[F]())
	if err != nil {
		return nil, 0, xerror.WrapWithXCode(err, xcode.DBRequestParamError)
	}

	var count int64
	if err := countDB.Count(&count).Error; err != nil {
		return nil, 0, xerror.WrapWithXCode(err, xcode.DBFailed)
	}

	if count == 0 {
		return []*M{}, 0, nil
	}

	findDB, err := q.buildQuery(ctx, query)
	if err != nil {
		return nil, 0, xerror.WrapWithXCode(err, xcode.DBRequestParamError)
	}

	if err := findDB.Find(&records).Error; err != nil {
		return nil, 0, xerror.WrapWithXCode(err, xcode.DBFailed)
	}

	return records, uint(count), nil
}

// CountBy 统计记录数量
func (q *searcher[M, F]) CountBy(ctx context.Context, filter *F) (count int64, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "searcher", "count_by", time.Since(start), err)
	}()

	db := q.db.WithContext(ctx).Model(new(M))
	if filter != nil {
		opt := dbquery.NewQuery(*filter)
		db, err = dbquery.Build(db, opt,
			dbquery.WithFilterTransfer[F](q.filterTransfer),
			dbquery.WithSortMapping[F](dbquery.NewSortMapping(dbquery.WithDefaultSort(""))),
		)
		if err != nil {
			return 0, xerror.WrapWithXCode(err, xcode.DBRequestParamError)
		}
	}

	if err := db.Count(&count).Error; err != nil {
		return 0, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return count, nil
}

// ExistsBy 判断记录是否存在（使用 SELECT 1 ... LIMIT 1，比 COUNT 更高效）
func (q *searcher[M, F]) ExistsBy(ctx context.Context, filter *F) (exists bool, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "searcher", "exists_by", time.Since(start), err)
	}()

	db := q.db.WithContext(ctx).Model(new(M))
	if filter != nil {
		opt := dbquery.NewQuery(*filter)
		db, err = dbquery.Build(db, opt,
			dbquery.WithFilterTransfer[F](q.filterTransfer),
			dbquery.WithSortMapping[F](dbquery.NewSortMapping(dbquery.WithDefaultSort(""))),
		)
		if err != nil {
			return false, xerror.WrapWithXCode(err, xcode.DBRequestParamError)
		}
	}

	result := db.Select("1").Limit(1).Scan(new(int))
	if result.Error != nil {
		return false, xerror.WrapWithXCode(result.Error, xcode.DBFailed)
	}
	return result.RowsAffected > 0, nil
}

// Find 通用查询方法
func (q *searcher[M, F]) Find(ctx context.Context, query dbquery.IQuery[F], optBuilders ...findOptionBuilder) (records []*M, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "searcher", "find", time.Since(start), err)
	}()

	opt := new(findOption)
	for _, build := range optBuilders {
		build(opt)
	}

	db, err := q.buildQuery(ctx, query)
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBRequestParamError)
	}

	for _, p := range opt.preloads {
		db = db.Preload(p)
	}
	if len(opt.selects) > 0 {
		db = db.Select(opt.selects)
	}
	if opt.limit > 0 {
		db = db.Offset(opt.start).Limit(opt.limit)
	}

	if err := db.Find(&records).Error; err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return records, nil
}

// FirstWith 带选项查询单条记录
func (q *searcher[M, F]) FirstWith(ctx context.Context, query dbquery.IQuery[F], optBuilders ...findOptionBuilder) (record *M, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "searcher", "first_with", time.Since(start), err)
	}()

	opt := new(findOption)
	for _, build := range optBuilders {
		build(opt)
	}

	db, err := q.buildQuery(ctx, query)
	if err != nil {
		return nil, xerror.WrapWithXCode(err, xcode.DBRequestParamError)
	}

	for _, p := range opt.preloads {
		db = db.Preload(p)
	}
	if len(opt.selects) > 0 {
		db = db.Select(opt.selects)
	}

	var r M
	if err := db.First(&r).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerror.NewXCode(xcode.DBRecordNotFound)
		}
		return nil, xerror.WrapWithXCode(err, xcode.DBFailed)
	}
	return &r, nil
}

// ListByCursor 游标分页查询记录
func (q *searcher[M, F]) ListByCursor(ctx context.Context, query dbquery.IQuery[F]) (records []*M, nextCursor pager.Cursor, err error) {
	start := time.Now()
	defer func() {
		recordDBRepoMetric(ctx, "searcher", "list_by_cursor", time.Since(start), err)
	}()

	db, err := q.buildQuery(ctx, query)
	if err != nil {
		return nil, "", xerror.WrapWithXCode(err, xcode.DBRequestParamError)
	}

	if err := db.Find(&records).Error; err != nil {
		return nil, "", xerror.WrapWithXCode(err, xcode.DBFailed)
	}

	if len(records) > 0 && q.cursorExtractor != nil {
		nextCursor = pager.Cursor(q.cursorExtractor(records[len(records)-1]))
	}

	return records, nextCursor, nil
}
