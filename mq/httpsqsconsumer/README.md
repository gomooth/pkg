# httpsqsconsumer — HTTPSQS 消费者

HTTP Simple Queue Service 消费者，嵌入 `queue.BaseConsumer` 提供重试退避和失败处理。

## 接口

```go
type IHandler interface {
    QueueName() string
    GetClient() (httpsqs.IClient, error)
    OnBefore(ctx context.Context) error
    Handle(ctx context.Context, data string, pos int64) error
    OnFailed(ctx context.Context, data string, err error)
}
```

## 使用示例

```go
type myHandler struct{}

func (h *myHandler) QueueName() string { return "my-queue" }
func (h *myHandler) GetClient() (httpsqs.IClient, error) {
    return httpsqs.NewClient("http://localhost:1218"), nil
}
func (h *myHandler) OnBefore(ctx context.Context) error { return nil }
func (h *myHandler) Handle(ctx context.Context, data string, pos int64) error {
    return processMessage(data)
}
func (h *myHandler) OnFailed(ctx context.Context, data string, err error) {
    slog.Error("failed", "data", data, "err", err)
}

consumer := httpsqsconsumer.New(
    httpsqsconsumer.WithHandler(&myHandler{}),
    httpsqsconsumer.WithBackoff(&retry.ExponentialDelay{Base: time.Minute, Max: 24 * time.Hour}),
)

if err := consumer.Consume(ctx); err != nil {
    log.Fatal(err)
}
```

## 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithHandler(handler)` | 消息处理器 | 必填 |
| `WithBackoff(strategy)` | 退避策略 | ExponentialDelay{Base:1min, Max:24h} |
| `WithLogger(log)` | 日志 | 控制台日志 |
| `WithEmptyQueueSleep(d)` | 空队列休眠时间 | 1s |
| `WithFailedCallbackDelay(d)` | 失败回调延迟 | 0 |
