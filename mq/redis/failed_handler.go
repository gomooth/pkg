package redis

import (
	"context"
	"log/slog"
)

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
func (h *defaultFailedHandler) Print(ctx context.Context, queue string, message []byte, err error) {
	args := []any{
		"component", "redis-consumer",
		"queue", queue,
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		args = append(args, "contextErr", ctxErr.Error())
	}
	if err != nil {
		args = append(args, "error", err.Error())
	}

	h.logger.Error("message consume failed", args...)
}
