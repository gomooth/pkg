package kafka

import (
	"context"

	"github.com/gomooth/pkg/mq/internal/logutil"
	"github.com/gomooth/pkg/mq/internal/types"
)

// DefaultFailedHandlerFunc 创建默认的失败处理回调函数。
// 记录消息处理失败日志，包含 consumerGroup、topic 和错误信息。
func DefaultFailedHandlerFunc(logger logutil.Logger) types.FailedHandlerFunc {
	return func(ctx context.Context, msg types.Message, err error) {
		if logger == nil {
			return
		}

		group, _ := msg.KafkaGroup()
		args := []any{
			"component", "kafka-consumer",
			"consumerGroup", group,
			"topic", msg.Queue,
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