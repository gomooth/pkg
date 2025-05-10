package dbcache

import (
	"context"

	"github.com/gomooth/pkg/framework/dbquery"
)

type IDBCache[E, F any] interface {
	// Paginate 分页列表
	Paginate(ctx context.Context, start, limit int, opt dbquery.IFilter[F], query func() ([]*E, uint, error)) ([]*E, uint, error)
	// List 列表所有
	List(ctx context.Context, opt dbquery.IFilter[F], query func() ([]*E, error)) ([]*E, error)
	// First 按 id 查询数据
	First(ctx context.Context, id uint, query func() (*E, error)) (*E, error)

	// Clear 清理所有缓存
	Clear(ctx context.Context, opts ...func(*clearOption)) error

	// Remember 缓存
	Remember(ctx context.Context, key string, query func() (any, error)) (any, error)
	// Forget 清理指定数据缓存
	Forget(ctx context.Context, key string) error
}
