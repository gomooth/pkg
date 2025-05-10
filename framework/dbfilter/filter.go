package dbfilter

import (
	"encoding/json"
	"fmt"

	"github.com/gomooth/pkg/framework/pager"

	"gorm.io/gorm"
)

// dbFilter 分页参数
type dbFilter[T any] struct {
	filter   *T
	sorters  []pager.Sorter
	preloads []string
}

func New[T any](filter *T, opts ...func(*option)) IDBFilter[T] {
	pOpt := new(option)
	for _, opt := range opts {
		opt(pOpt)
	}

	return &dbFilter[T]{
		filter:   filter,
		sorters:  pOpt.sorters,
		preloads: pOpt.preloads,
	}
}

func (p *dbFilter[T]) Filter() *T {
	if p.filter == nil {
		p.filter = new(T)
	}
	return p.filter
}

func (p *dbFilter[T]) Sorters() []pager.Sorter {
	if p.sorters == nil {
		return []pager.Sorter{}
	}

	return p.sorters
}

func (p *dbFilter[T]) Preloads() []string {
	if p.preloads == nil {
		return []string{}
	}

	return p.preloads
}

func (p *dbFilter[T]) Build(db *gorm.DB, opts ...func(*buildOption)) *gorm.DB {
	if p.filter == nil {
		p.filter = new(T)
	}

	cnf := &buildOption{
		useDefaultSorter: true,
		sortFields:       make(map[string]string),
	}
	for _, opt := range opts {
		opt(cnf)
	}

	if p.preloads != nil {
		for _, preload := range p.preloads {
			db = db.Preload(preload)
		}
	}

	if p.sorters != nil {
		for _, sorter := range p.sorters {
			field, ok := cnf.sortFields[sorter.Field]
			if !ok {
				field = sorter.Field
			}
			db = db.Order(fmt.Sprintf("%s %s", field, sorter.Sorted))
		}
	}
	if cnf.useDefaultSorter {
		db = db.Order("id DESC")
	}

	return db
}

func (p *dbFilter[T]) String() string {
	data := map[string]interface{}{
		"filter":   p.filter,
		"sorters":  p.sorters,
		"preloads": p.preloads,
	}
	bs, _ := json.Marshal(data)
	return string(bs)
}
