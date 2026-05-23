package dbquery

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gomooth/pkg/framework/pager"
	"github.com/gomooth/utils/strutil"
)

// ISortSpec 定义查询的排序维度。
// 独立于过滤和分页，排序意图（Sorters）和排序约束（SortMapping）解耦：
//   - ISortSpec 携带用户意图（哪些字段、什么方向）
//   - SortMapping 携带后端知识（字段映射、白名单、校验策略、默认排序）
type ISortSpec interface {
	// Sorters 返回用户指定的排序规则列表
	Sorters() []pager.Sorter
	// String 生成排序部分的缓存键
	String() string
}

// sortSpec ISortSpec 的默认实现
type sortSpec struct {
	sorters []pager.Sorter
}

func (s *sortSpec) Sorters() []pager.Sorter {
	if s.sorters == nil {
		return []pager.Sorter{}
	}
	return s.sorters
}

func (s *sortSpec) String() string {
	if len(s.sorters) == 0 {
		return ""
	}
	type sorterJSON struct {
		Field  string `json:"field"`
		Sorted string `json:"sorted"`
	}
	items := make([]sorterJSON, len(s.sorters))
	for i, s := range s.sorters {
		items[i] = sorterJSON{Field: s.Field, Sorted: s.Sorted.String()}
	}
	bs, _ := json.Marshal(items)
	return string(bs)
}

// NewSortSpec 创建排序规格
func NewSortSpec(sorters []pager.Sorter) ISortSpec {
	return &sortSpec{sorters: sorters}
}

// SortMapping 排序字段映射，封装字段白名单 + 映射关系 + 校验策略。
// 将散落在 searcher（sortKeyMapping）和 gormbuild（校验逻辑）中的排序知识收敛为一处。
type SortMapping struct {
	fields       map[string]string // 前端字段名 → 数据库列名
	strict       bool              // 严格模式：未知字段返回错误
	defaultField string            // 默认排序字段（需在 fields 白名单中）
	defaultDir   string            // 默认排序方向 ASC/DESC
}

// NewSortMapping 创建排序映射
func NewSortMapping(opts ...func(*SortMapping)) *SortMapping {
	m := &SortMapping{
		fields:       make(map[string]string),
		defaultField: "id",
		defaultDir:   "DESC",
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Resolve 将前端排序字段解析为数据库列名。
// 返回 (数据库列名, 是否存在于白名单)。
func (m *SortMapping) Resolve(field string) (column string, ok bool) {
	col, ok := m.fields[field]
	return col, ok
}

// DefaultSort 返回默认排序语句，如 "id DESC"。
// 默认字段必须在白名单中，否则回退到 "id DESC"。
func (m *SortMapping) DefaultSort() string {
	col, ok := m.fields[m.defaultField]
	if !ok {
		return "id DESC"
	}
	return fmt.Sprintf("%s %s", col, m.defaultDir)
}

// IsStrict 返回是否为严格排序校验模式
func (m *SortMapping) IsStrict() bool {
	return m.strict
}

// --- SortMapping 选项函数 ---

// WithSortFields 注册允许排序的数据库字段名（字段名即列名，无映射）
func WithSortFields(field string, others ...string) func(*SortMapping) {
	return func(m *SortMapping) {
		fields := append([]string{field}, others...)
		for _, key := range fields {
			m.fields[key] = key
		}
	}
}

// WithSortKeyMap 注册前端排序字段到数据库列名的映射关系
func WithSortKeyMap(mapping map[string]string) func(*SortMapping) {
	return func(m *SortMapping) {
		for key, field := range mapping {
			vkey := strutil.Snake(key)
			m.fields[vkey] = field
		}
	}
}

// WithStrictSort 启用严格排序字段校验。
// 当 true 时，未知排序字段返回错误而非静默跳过；默认关闭。
func WithStrictSort(strict bool) func(*SortMapping) {
	return func(m *SortMapping) {
		m.strict = strict
	}
}

// WithDefaultSort 设置默认排序字段和方向。
// 字段必须已通过 WithSortFields 或 WithSortKeyMap 注册到白名单中，否则回退到 "id DESC"。
// dir 仅接受 "ASC" 或 "DESC"（不区分大小写），默认 "DESC"。
func WithDefaultSort(field string, dir ...string) func(*SortMapping) {
	return func(m *SortMapping) {
		if len(field) == 0 {
			return
		}
		d := "DESC"
		if len(dir) > 0 && strings.EqualFold(dir[0], "ASC") {
			d = "ASC"
		}
		m.defaultField = field
		m.defaultDir = d
	}
}
