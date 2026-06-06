package engine

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"
)

// 引擎状态常量
const (
	Idle         int32 = 0
	Running      int32 = 1
	ShuttingDown int32 = 2
	Closed       int32 = 3
)

// Base 共享引擎基础结构，各 MQ consumerEngine 内嵌此结构复用生命周期管理
type Base struct {
	State       atomic.Int32
	CancelFunc  context.CancelFunc
	WG          sync.WaitGroup
	Logger      *slog.Logger
	Metrics     interface{} // 使用 interface{} 避免循环导入 mq/internal/metrics
	PanicHandler func(any)
}

// HealthCheck 检查引擎是否处于运行状态
func (b *Base) HealthCheck(_ context.Context) error {
	if b.State.Load() != Running {
		return xerror.NewXCode(xcode.ErrMQConsume,
			fmt.Sprintf("consumer not running (state=%d)", b.State.Load()))
	}
	return nil
}

// SafeGo 启动带 panic 恢复的 goroutine
func (b *Base) SafeGo(name string, fn func(), panicHandler func(any)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if b.Logger != nil {
					b.Logger.Error("goroutine panic recovered",
						"name", name,
						"panic", r,
						"stack", string(debug.Stack()),
					)
				}
				if panicHandler != nil {
					panicHandler(r)
				} else if b.PanicHandler != nil {
					b.PanicHandler(r)
				}
			}
		}()
		fn()
	}()
}

// TryStart 尝试从 Idle 切换到 Running（CAS）
func (b *Base) TryStart() bool {
	return b.State.CompareAndSwap(Idle, Running)
}

// RequestShutdown 尝试从 Running 切换到 ShuttingDown（CAS）
func (b *Base) RequestShutdown() bool {
	return b.State.CompareAndSwap(Running, ShuttingDown)
}
