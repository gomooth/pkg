package retry

import (
	"context"

	"github.com/gomooth/pkg/mq/internal/types"
)

// RetryStrategy 重试策略接口
type RetryStrategy interface {
	OnMessage(ctx context.Context, msg types.Message, handle func(ctx context.Context, msg types.Message) error) error
	SetFailedHandler(fn types.FailedHandlerFunc)
	SetDeadLetterHandler(h types.DeadLetterHandler)
}
