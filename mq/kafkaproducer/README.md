# kafkaproducer — Kafka 生产者

基于 Sarama 的 Kafka 消息生产者，支持单条发送、批量发送和顺序发送。

## 接口

```go
type IProducer interface {
    // Produce 发送单条消息
    Produce(ctx context.Context, topic string, message []byte) error

    // Produces 批量发送多条消息
    Produces(ctx context.Context, topic string, message ...[]byte) error

    // ProduceWithSequence 按顺序发送消息（基于 sequenceKey 分区）
    ProduceWithSequence(ctx context.Context, topic, sequenceKey string, messages ...[]byte) error

    // Close 关闭生产者
    Close() error
}
```

## 使用示例

```go
producer, err := kafkaproducer.New(
    []string{"localhost:9092"},
    kafkaproducer.WithTimeout(10*time.Second),
)
if err != nil {
    log.Fatal(err)
}
defer producer.Close()

// 单条发送
err = producer.Produce(ctx, "orders", []byte(`{"id":1}`))

// 批量发送
err = producer.Produces(ctx, "orders",
    []byte(`{"id":1}`),
    []byte(`{"id":2}`),
)

// 顺序发送（同一 sequenceKey 路由到同一分区，保证顺序）
err = producer.ProduceWithSequence(ctx, "orders", "user-123",
    []byte(`{"id":1}`),
    []byte(`{"id":2}`),
)
```

## 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithLogger(logger)` | 日志 | 控制台日志 |
| `WithTimeout(d)` | 发送超时 | 5s |

## 内部配置

- Sarama 版本：V3_6_0_0
- 压缩：ZSTD
- 分区策略：RoundRobin
- 批量刷新：10 条或 500ms
