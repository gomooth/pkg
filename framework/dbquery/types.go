package dbquery

import (
	"github.com/gomooth/pkg/framework/pager"
	"gorm.io/gorm"
)

// IFilter DB 分页参数
type IFilter[T any] interface {
	pager.IPager[T]

	Preloads() []string

	Build(db *gorm.DB, opts ...func(*buildOption)) *gorm.DB
}
