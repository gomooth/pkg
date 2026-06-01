package job

import (
	"context"

	"github.com/robfig/cron/v3"
)

// ICommandJob 命令式任务接口，定义任务的执行方法
type ICommandJob interface {
	Run(ctx context.Context, args ...string) error
}

// IWrapper 任务包装器接口，将 ICommandJob 转换为 cron.Job
type IWrapper interface {
	FromCommandJob(ctx context.Context, job ICommandJob, args ...string) cron.Job
}

// ICronjobRegister Cron 定时任务注册接口
type ICronjobRegister interface {
	Register(ctx context.Context, spec string, cmd ICommandJob)
}

// ICommandRegister 命令行任务注册接口
type ICommandRegister interface {
	Register(ctx context.Context, name string, cmd ICommandJob)
}
