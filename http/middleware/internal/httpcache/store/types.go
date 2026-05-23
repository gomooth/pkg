package store

import (
	"context"
	"net/http"
	"time"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"
)

var ErrorCacheMiss = xerror.NewXCode(xcode.ErrCacheMiss, "cache miss error")

type ICacheStore interface {
	// Get 获取缓存，如果未获取到，返回 ErrorCacheMiss 错误
	Get(ctx context.Context, key string, value *CachedResponse) error

	// Set 设置缓存，如果存在，则覆盖
	Set(ctx context.Context, key string, value *CachedResponse, expire time.Duration) error

	// Delete 删除缓存，如果不存在，则不处理
	Delete(ctx context.Context, key string) error
}

// CachedResponse 缓存的响应
// 注意：此类型位于 internal 包中，外部无法直接引用。
// 若需让外部实现 ICacheStore，需将此类型移至公开包或改用 any 接口参数。
type CachedResponse struct {
	Status int
	Header http.Header

	Data []byte
}
