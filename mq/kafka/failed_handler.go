package kafka

import (
	"context"

	"github.com/gomooth/pkg/mq/internal/logutil"
)

// FailedHandlerFunc 失败处理回调函数类型
type FailedHandlerFunc func(ctx context.Context, consumerGroup string, topic string, message []byte, err error)

// DefaultFailedHandlerFunc 创建默认的失败处理回调函数。
// 记录消息处理失败日志，包含 consumerGroup、topic 和错误信息。
func DefaultFailedHandlerFunc(logger logutil.Logger) FailedHandlerFunc {
	return func(ctx context.Context, consumerGroup string, topic string, message []byte, err error) {
		if logger == nil {
			return
		}
		args := []any{
			"component", "kafka-consumer",
			"consumerGroup", consumerGroup,
			"topic", topic,
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			args = append(args, "contextErr", ctxErr.Error())
		}
		if err != nil {
			args = append(args, "error", err.Error())
		}

		logger.Error("message consume failed", args...)
	}
}
