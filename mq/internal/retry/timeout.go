package retry

import (
	"context"
	"time"
)

// ApplyTimeout 为函数调用添加超时控制。
// timeout <= 0 时不添加超时，直接执行。
func ApplyTimeout(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
	if timeout <= 0 {
		return fn(ctx)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(ctx)
}
