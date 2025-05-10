package dbquery

import "github.com/gomooth/pkg/framework/pager"

type option struct {
	sorters  []pager.Sorter
	preloads []string
}

// WithSorts 排序规则
// 以符号开头，可选符号：(+或空 正序）（- 倒序）（* 自定义复杂排序标识关键词）
// 多个排序规则按英文逗号隔开
func WithSorts(sort string) func(*option) {
	return func(opt *option) {
		opt.sorters = pager.ParseSorts(sort)
	}
}

// WithPreloads gorm Preload
func WithPreloads(preload string, others ...string) func(*option) {
	return func(opt *option) {
		opt.preloads = append([]string{preload}, others...)
	}
}
