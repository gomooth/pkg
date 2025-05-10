package dbfilter

import (
	"github.com/gomooth/pkg/framework/pager"
	"gorm.io/gorm"
)

// IDBFilter DB 分页参数
type IDBFilter[T any] interface {
	pager.IPager[T]

	Preloads() []string

	Build(db *gorm.DB, opts ...func(*buildOption)) *gorm.DB
}
