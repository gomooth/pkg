# job — 定时任务

提供 Cron 包装器和命令式任务，支持自动重试和失败记录。

## 组件

### Cron 包装器

将 `ICommandJob` 注册为 Cron 定时任务，自动处理重试和日志记录。

```go
wrapper := job.NewCronWrapper(log)
wrapper.RegisterFunc("0 * * * *", myJob)
wrapper.Start()
```

### 命令式任务

`ICommandJob` 接口定义任务执行逻辑，内置重试机制：

```go
type MyJob struct{}

func (j *MyJob) Run() error {
    return doWork()
}

// 任务执行失败时自动重试，重试耗尽后记录失败
cmdJob := job.NewCommandJob(&MyJob{}, job.WithMaxRetry(3))
err := cmdJob.Run()
```

### 接口

| 接口 | 说明 |
|------|------|
| `ICommandJob` | 命令式任务，实现 `Run() error` |
| `IWrapper` | Cron 包装器，注册和调度任务 |
