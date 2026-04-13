package gormbuild

import (
	"fmt"

	"gorm.io/gorm"
)

func Build[F any](db *gorm.DB, opts ...func(*option[F])) *gorm.DB {
	cnf := &option[F]{
		useDefaultSorter: true,
		sortFields:       make(map[string]string),
	}
	for _, opt := range opts {
		opt(cnf)
	}

	if cnf.filter == nil {
		return db
	}

	if cnf.filterTransfer != nil {
		db = cnf.filterTransfer(cnf.filter.Filter(), db)
	}

	if cnf.filter.Preloads() != nil {
		for _, preload := range cnf.filter.Preloads() {
			db = db.Preload(preload)
		}
	}

	if cnf.filter.Sorters() != nil {
		for _, sorter := range cnf.filter.Sorters() {
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
