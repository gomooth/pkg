package dbquery

import (
	"github.com/gomooth/pkg/framework/pager"
	"github.com/gomooth/utils/strutil"
)

type buildOption struct {
	usePage          bool
	start, limit     int
	useDefaultSorter bool
	sortFields       map[string]string
}

// BuildWithPage 分页参数，默认最大 limit 不超过100
func BuildWithPage(start, limit int) func(*buildOption) {
	return func(o *buildOption) {
		o.usePage = true
		if limit <= 0 {
			limit = pager.DefaultPageSize
		}
		// 默认最多返回 100，除非主动修改
		o.limit = min(limit, 100)
		o.start = start
	}
}

// BuildWithLimitPage 限制最大条数的分页参数
func BuildWithLimitPage(start, limit, maxLimit int) func(*buildOption) {
	return func(o *buildOption) {
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

// BuildWithDefaultSorter 是否启用默认ID倒序排序。默认启用
func BuildWithDefaultSorter(enabled bool) func(*buildOption) {
	return func(o *buildOption) {
		o.useDefaultSorter = enabled
	}
}

// BuildWithSortField 指定启用排序的数据库字段。需保证和数据库中字段一致
func BuildWithSortField(field string, other ...string) func(*buildOption) {
	return func(o *buildOption) {
		if o.sortFields == nil {
			o.sortFields = make(map[string]string)
		}
		fields := append([]string{field}, other...)
		for _, key := range fields {
			o.sortFields[key] = key
		}
	}
}

// BuildWithSortKeyMappings 指定前端排序字段和数据库字段的映射关系。
func BuildWithSortKeyMappings(mapping map[string]string) func(*buildOption) {
	return func(o *buildOption) {
		if o.sortFields == nil {
			o.sortFields = make(map[string]string)
		}
		for key, field := range mapping {
			vkey := strutil.Snake(key)
			o.sortFields[vkey] = field
		}
	}
}
