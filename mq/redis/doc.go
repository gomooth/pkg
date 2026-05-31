// Package redis 提供统一的 Redis 队列消费者和生产者实现。
//
// 消费者支持同步重试和再入队重试两种模式，通过可配置的退避策略控制重试节奏。
//
// 生产者支持单条和批量推送，内置 Pipeline 优化。
//
// Consumer 和 Producer 均实现 app.IApp 接口，可通过 app.Manager 统一管理生命周期。
package redis
