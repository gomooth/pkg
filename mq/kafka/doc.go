// Package kafka 提供统一的 Kafka 消费者和生产者实现。
//
// 消费者支持同步和异步两种重试模式，异步模式可通过可插拔的 RetryStore
// 选择内存水位线存储或 Redis 持久化存储。
//
// 生产者支持单条、批量无序和有序三种发送模式，内置自动重连机制。
//
// Consumer 和 Producer 均实现 app.IApp 接口，可通过 app.Manager 统一管理生命周期。
package kafka
