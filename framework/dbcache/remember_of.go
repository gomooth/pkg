package dbcache

import (
	"context"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"
)

// RememberOf 缓存查询结果并返回具体类型，query 可感知 context 取消。
func RememberOf[T, E, F any](ctx context.Context, c IDBCache[E, F], key string, query func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	codec := c.Codec()

	data, err := c.Remember(ctx, key, func(ctx context.Context) ([]byte, error) {
		result, err := query(ctx)
		if err != nil {
			return nil, err
		}
		return codec.Marshal(result)
	})
	if err != nil {
		return zero, err
	}

	var typed T
	if err := codec.Unmarshal(data, &typed); err != nil {
		return zero, xerror.WrapWithXCode(err, xcode.ErrCacheReadFailed)
	}
	return typed, nil
}
