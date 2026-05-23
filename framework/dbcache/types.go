package dbcache

import (
	"context"

	"github.com/gomooth/pkg/framework/dbquery"
)

// IQueryCache 按条件获取实体的缓存结果
type IQueryCache[E, F any] interface {
	// First 按 id 查询数据
	First(ctx context.Context, id uint, query func(ctx context.Context) (*E, error)) (*E, error)
	// List 列表所有
	List(ctx context.Context, q dbquery.IQuery[F], query func(ctx context.Context) ([]*E, error)) ([]*E, error)
	// Paginate 分页列表
	Paginate(ctx context.Context, q dbquery.IQuery[F], query func(ctx context.Context) ([]*E, uint, error)) ([]*E, uint, error)
}

// IKeyValueCache 键值对缓存操作
type IKeyValueCache interface {
	// Remember 缓存查询结果，query 可感知 context 取消
	Remember(ctx context.Context, key string, query func(ctx context.Context) ([]byte, error)) ([]byte, error)
	// Forget 清理指定数据缓存
	Forget(ctx context.Context, key string) error
	// Codec 返回缓存使用的编解码器
	Codec() Codec
}

// ICacheManager 批量缓存失效操作
type ICacheManager interface {
	// Clear 清理所有缓存
	Clear(ctx context.Context, opts ...func(*clearOption)) error
}

// IDBCache 数据库缓存组合接口
type IDBCache[E, F any] interface {
	IQueryCache[E, F]
	IKeyValueCache
	ICacheManager
}
