package retry

import (
	"context"

	"github.com/gomooth/pkg/mq/internal/metrics"
	"github.com/gomooth/pkg/mq/internal/types"
)

// HandleExhausted 处理重试耗尽的消息。
// 优先调用 DeadLetterHandler，若无则调用 FailedHandlerFunc。
func HandleExhausted(
	ctx context.Context,
	m *metrics.ConsumerMetrics,
	deadLetter types.DeadLetterHandler,
	failed types.FailedHandlerFunc,
	msg types.Message,
	lastErr error,
) {
	if m != nil {
		m.OnDeadLetter()
	}
	if deadLetter != nil {
		_ = deadLetter.OnDeadLetter(ctx, msg, lastErr)
		return
	}
	if failed != nil {
		failed(ctx, msg, lastErr)
	}
}
