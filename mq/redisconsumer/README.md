# redisconsumer — Redis 队列消费者

基于 Redis List 的消息消费者，嵌入 `queue.BaseConsumer` 提供重试退避和失败处理。

## 使用示例

```go
consumer := redisconsumer.New(
    redisconsumer.WithHandler(&queue.RedisQueueConfig{
        Addr: "localhost:6379",
    }, "my-queue", func(val string) error {
        return processMessage(val)
    }),
    redisconsumer.WithFailedHandler(func(val string, err error) {
        slog.Error("failed to process", "val", val, "err", err)
    }),
    redisconsumer.WithBackoff(&retry.FixedDelay{Wait: 2 * time.Second}),
)

if err := consumer.Consume(ctx); err != nil {
    log.Fatal(err)
}
```

## 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithHandler(config, queueName, fn)` | Redis 配置 + 队列名 + 处理函数 | 必填 |
| `WithFailedHandler(fn)` | 失败处理回调 | nil |
| `WithBackoff(strategy)` | 重试退避策略 | FixedDelay{Wait: 1s} |
| `WithLogger(log)` | 日志 | 控制台日志 |
| `WithEmptyQueueSleep(d)` | 空队列休眠时间 | 1s |
| `WithFailedCallbackDelay(d)` | 失败回调延迟 | 0 |

## 工作原理

使用 `BLMOVE` 从 Redis List 阻塞消费消息，处理失败时按退避策略重试，重试耗尽后调用 `FailedHandler`。
