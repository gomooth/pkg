package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/gomooth/pkg/mq/internal/consume"
	"github.com/gomooth/pkg/mq/redis/internal"
	"github.com/redis/go-redis/v9"
)

// redisFetcher 实现 consume.Fetcher 接口，从 Redis 队列拉取消息
type redisFetcher struct {
	client     *redis.Client
	queueKey   string
	backupKey  string
	popTimeout int64
}

func newRedisFetcher(client *redis.Client, queueKey string, popTimeout int64) *redisFetcher {
	return &redisFetcher{
		client:     client,
		queueKey:   queueKey,
		backupKey:  fmt.Sprintf("%s_backup", queueKey),
		popTimeout: popTimeout,
	}
}

// Fetch 从 Redis 队列拉取一条消息
func (f *redisFetcher) Fetch(ctx context.Context) consume.FetchResult {
	timeout := time.Duration(f.popTimeout) * time.Second
	val, err := internal.PopScript.Run(ctx, f.client, []string{f.queueKey, f.backupKey}, timeout.Seconds()).Text()
	if err != nil {
		// Redis NIL 表示队列为空（BLMOVE 超时）
		if err == redis.Nil {
			return consume.FetchResult{Empty: true}
		}
		// 上下文取消
		if ctx.Err() != nil {
			return consume.FetchResult{Empty: true}
		}
		return consume.FetchResult{Err: err}
	}
	if val == "" {
		return consume.FetchResult{Empty: true}
	}
	return consume.FetchResult{Data: val}
}
