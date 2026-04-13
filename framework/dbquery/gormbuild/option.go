package gormbuild

import (
	"github.com/gomooth/pkg/framework/dbfilter"
	"github.com/gomooth/pkg/framework/pager"
	"github.com/gomooth/utils/strutil"
	"gorm.io/gorm"
)

type builder[F any] func(filter dbfilter.IFilter[F], db *gorm.DB) *gorm.DB

type option[F any] struct {
	filter           dbfilter.IFilter[F]
	filterTransfer   func(filter *F, db *gorm.DB) *gorm.DB
	usePage          bool
	start, limit     int
	useDefaultSorter bool
	sortFields       map[string]string
}

func WithFilter[F any](filter dbfilter.IFilter[F], transfer func(filter *F, db *gorm.DB) *gorm.DB) func(*option[F]) {
	return func(o *option[F]) {
		o.filter = filter
		o.filterTransfer = transfer
	}
}

// WithPage 分页参数，默认最大 limit 不超过100
func WithPage[F any](start, limit int) func(*option[F]) {
	return func(o *option[F]) {
		o.usePage = true
		if limit <= 0 {
			limit = pager.DefaultPageSize
		}
		// 默认最多返回 100，除非主动修改
		o.limit = min(limit, 100)
		o.start = start
	}
}

// WithLimitPage 限制最大条数的分页参数
func WithLimitPage[F any](start, limit, maxLimit int) func(*option[F]) {
	return func(o *option[F]) {
		if limit <= 0 {
			limit = pager.DefaultPageSize
		}

		// 默认最多返回 100，除非主动修改
		if maxLimit == 0 {
			maxLimit = 100
		}
		o.limit = min(limit, maxLimit)
		o.start = start
	}
}

// WithDefaultSorter 是否启用默认ID倒序排序。默认启用
func WithDefaultSorter[F any](enabled bool) func(*option[F]) {
	return func(o *option[F]) {
		o.useDefaultSorter = enabled
	}
}

// WithSortField 指定启用排序的数据库字段。需保证和数据库中字段一致
func WithSortField[F any](field string, other ...string) func(*option[F]) {
	return func(o *option[F]) {
		if o.sortFields == nil {
			o.sortFields = make(map[string]string)
		}
		fields := append([]string{field}, other...)
		for _, key := range fields {
			o.sortFields[key] = key
		}
	}
}

// WithSortKeyMappings 指定前端排序字段和数据库字段的映射关系。
func WithSortKeyMappings[F any](mapping map[string]string) func(*option[F]) {
	return func(o *option[F]) {
		if o.sortFields == nil {
			o.sortFields = make(map[string]string)
		}
		for key, field := range mapping {
			vkey := strutil.Snake(key)
			o.sortFields[vkey] = field
		}
	}
}
