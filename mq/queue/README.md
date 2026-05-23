# queue — 通用队列接口

定义队列的基本操作接口、消费者框架和 Redis 队列实现。

## 核心接口

### IQueue — 队列操作

```go
type IQueue interface {
    Push(ctx context.Context, value string) error
    Pop(ctx context.Context) (string, error)
    Close() error
}
```

### IHandler — 消息处理器

```go
type IHandler interface {
    QueueName() string
    OnBefore(ctx context.Context) error
    Handle(ctx context.Context, data string) error
    OnFailed(ctx context.Context, data string, err error)
}
```

`FuncHandler` 适配函数到 `IHandler` 接口：

```go
handler := queue.FuncHandler("my-queue", processFunc, onFailedFunc)
```

### Fetcher — 消息源

```go
type Fetcher interface {
    Fetch(ctx context.Context) (string, error)
}
```

### BaseConsumer — 通用消费者

内置重试退避、空队列休眠、失败回调延迟，可嵌入到具体消费者实现中。

```go
type myConsumer struct {
    queue.BaseConsumer
}

func (c *myConsumer) Consume(ctx context.Context) error {
    return c.BaseConsumer.Consume(ctx)
}
```

### IConsumeServer — 消费者服务

```go
type IConsumeServer interface {
    app.IApp
    IRegister
}
```

## Redis 队列

使用 `LPUSH` + `BLMOVE` 原子操作实现可靠消息传递，消费中的消息转移到备份队列，处理完成后再移除。

```go
q := queue.NewSimpleRedis(&queue.RedisQueueConfig{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
    Timeout:  5 * time.Second,
}, "my-queue")

err := q.Push(ctx, "hello")
val, err := q.Pop(ctx)
```

## 消费者服务

```go
srv := queue.NewServer(
    queue.WithMaxRestartPerConsumer(5),
    queue.WithOnConsumerError(func(ce queue.ConsumerError) {
        slog.Error("consumer error", "err", ce.Err, "attempts", ce.Attempts)
    }),
)

srv.Register(consumer1)
srv.Register(consumer2)
srv.Run(context.Background())
```

服务自动管理消费者生命周期，消费协程异常退出时按退避策略自动重启。

## 配置选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `WithMaxRestartPerConsumer(n)` | 单个消费者最大重启次数 | 10 |
| `WithOnConsumerError(fn)` | 消费者错误回调 | nil |
| `WithRestartBackoff(strategy)` | 重启退避策略 | ExponentialDelay{Base:5s, Max:5min} |
