package kafka

import (
	"context"
	"log/slog"
)

// FailedHandlerFunc 失败处理回调函数类型
type FailedHandlerFunc func(ctx context.Context, consumerGroup string, topic string, message []byte, err error)

// defaultFailedHandler 默认失败处理器
type defaultFailedHandler struct {
	logger *slog.Logger
}

func newDefaultFailedHandler(logger *slog.Logger) *defaultFailedHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &defaultFailedHandler{logger: logger}
}

// Print 记录消息处理失败日志
// 修复 P15：移除 ctx!=nil 冗余检查（Go context 永远不为 nil）
func (h *defaultFailedHandler) Print(ctx context.Context, consumerGroup string, topic string, message []byte, err error) {
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

	h.logger.Error("message consume failed", args...)
}
