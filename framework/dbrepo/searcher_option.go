package dbrepo

import "gorm.io/gorm"

type builderOptionBuilder[M any, F any] func(*searcher[M, F])

func WithFilterTransfer[M any, F any](transfer func(f *F, db *gorm.DB) *gorm.DB) builderOptionBuilder[M, F] {
	return func(q *searcher[M, F]) {
		q.filterTransfer = transfer
	}
}

func WithSortKeyMap[M any, F any](mapping map[string]string) builderOptionBuilder[M, F] {
	return func(q *searcher[M, F]) {
		q.sortKeyMapping = mapping
	}
}

type findOption struct {
	preloads []string
	selects  []string
	start    int
	limit    int
}

type findOptionBuilder func(*findOption)

// WithPreload 预加载关联
func WithPreload(preloads ...string) findOptionBuilder {
	return func(opt *findOption) {
		opt.preloads = preloads
	}
}

func WithSelect(selects ...string) findOptionBuilder {
	return func(opt *findOption) {
		if opt.selects == nil {
			opt.selects = make([]string, 0, len(selects))
		}
		opt.selects = append(opt.selects, selects...)
	}
}

//func WithSortFlag(flag string) findOptionBuilder {
//	return func(opt *findOption) {
//		opt.sortFlag = flag
//	}
//}
//
//func WithLimit(limit int) findOptionBuilder {
//	return func(opt *findOption) {
//		opt.limit = limit
//	}
//}
//
//func WithStart(start int) findOptionBuilder {
//	return func(opt *findOption) {
//		opt.start = start
//	}
//}

//// QueryOption 查询选项
//type QueryOption func(*gorm.DB) *gorm.DB
//
//// WithPreload 预加载关联
//func WithPreload(preloads ...string) QueryOption {
//	return func(db *gorm.DB) *gorm.DB {
//		for _, preload := range preloads {
//			db = db.Preload(preload)
//		}
//		return db
//	}
//}
//
//// WithSelect 选择字段
//func WithSelect(selects ...string) QueryOption {
//	return func(db *gorm.DB) *gorm.DB {
//		return db.Select(selects)
//	}
//}
//
//// WithOrder 排序
//func WithOrder(order string) QueryOption {
//	return func(db *gorm.DB) *gorm.DB {
//		return db.Order(order)
//	}
//}
//
//// WithLimit 限制数量
//func WithLimit(limit int) QueryOption {
//	return func(db *gorm.DB) *gorm.DB {
//		return db.Limit(limit)
//	}
//}
//
//// WithOffset 偏移量
//func WithOffset(offset int) QueryOption {
//	return func(db *gorm.DB) *gorm.DB {
//		return db.Offset(offset)
//	}
//}
