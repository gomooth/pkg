package internal

import (
	"runtime"

	"github.com/redis/go-redis/v9"
)

// BuildConsumerOptions 构建 Redis 消费者连接配置
// ReadTimeout 设为 0 以支持 BLMOVE 长阻塞
func BuildConsumerOptions(addr string, opts ...func(*redis.Options)) *redis.Options {
	cfg := &redis.Options{
		Addr:         addr,
		ReadTimeout:  0, // BLMOVE 需要长阻塞
		WriteTimeout: 0,
		PoolSize:     runtime.GOMAXPROCS(0) * 2,
		MinIdleConns: 2,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

// BuildProducerOptions 构建 Redis 生产者连接配置
func BuildProducerOptions(addr string, opts ...func(*redis.Options)) *redis.Options {
	cfg := &redis.Options{
		Addr:         addr,
		ReadTimeout:  0,
		WriteTimeout: 0,
		PoolSize:     runtime.GOMAXPROCS(0) * 2,
		MinIdleConns: 2,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}
