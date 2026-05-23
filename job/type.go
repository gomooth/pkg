package job

import (
	"context"

	"github.com/robfig/cron/v3"
)

type ICommandJob interface {
	Run(ctx context.Context, args ...string) error
}

type IWrapper interface {
	FromCommandJob(ctx context.Context, job ICommandJob, args ...string) cron.Job
}

type ICronjobRegister interface {
	Register(ctx context.Context, spec string, cmd ICommandJob)
}

type ICommandRegister interface {
	Register(ctx context.Context, name string, cmd ICommandJob)
}
