package dbquery

import (
	"encoding/json"
	"fmt"

	"github.com/gomooth/pkg/framework/pager"

	"gorm.io/gorm"
)

// dbQuery 分页参数
type dbQuery[T any] struct {
	filter   *T
	sorters  []pager.Sorter
	preloads []string
}

func New[T any](filter *T, opts ...func(*option)) IFilter[T] {
	pOpt := new(option)
	for _, opt := range opts {
		opt(pOpt)
	}

	return &dbQuery[T]{
		filter:   filter,
		sorters:  pOpt.sorters,
		preloads: pOpt.preloads,
	}
}

func (df *dbQuery[T]) Filter() *T {
	if df.filter == nil {
		df.filter = new(T)
	}
	return df.filter
}

func (df *dbQuery[T]) Sorters() []pager.Sorter {
	if df.sorters == nil {
		return []pager.Sorter{}
	}

	return df.sorters
}

func (df *dbQuery[T]) Preloads() []string {
	if df.preloads == nil {
		return []string{}
	}

	return df.preloads
}

func (df *dbQuery[T]) Build(db *gorm.DB, opts ...func(*buildOption)) *gorm.DB {
	if df.filter == nil {
		df.filter = new(T)
	}

	cnf := &buildOption{
		useDefaultSorter: true,
		sortFields:       make(map[string]string),
	}
	for _, opt := range opts {
		opt(cnf)
	}

	if df.preloads != nil {
		for _, preload := range df.preloads {
			db = db.Preload(preload)
		}
	}

	if df.sorters != nil {
		for _, sorter := range df.sorters {
			field, ok := cnf.sortFields[sorter.Field]
			if !ok {
				field = sorter.Field
			}
			db = db.Order(fmt.Sprintf("%s %s", field, sorter.Sorted))
		}
	}
	if cnf.usePage {
		db = db.Offset(cnf.start).Limit(cnf.limit)
	}
	if cnf.useDefaultSorter {
		db = db.Order("id DESC")
	}

	return db
}

func (df *dbQuery[T]) String() string {
	data := map[string]interface{}{
		"filter":   df.filter,
		"sorters":  df.sorters,
		"preloads": df.preloads,
	}
	bs, _ := json.Marshal(data)
	return string(bs)
}
