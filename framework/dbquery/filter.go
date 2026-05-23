package dbquery

// IFilter 定义查询的过滤维度（WHERE 条件 + GORM Preload）。
// 与旧版 dbfilter.IFilter 不同，此接口不嵌入 IPager，不包含排序和分页。
type IFilter[F any] interface {
	// Filter 返回过滤条件结构体指针
	Filter() *F
	// Preloads 返回 GORM 关联预加载列表
	Preloads() []string
}
