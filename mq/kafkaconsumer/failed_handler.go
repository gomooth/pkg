package kafkaconsumer

import (
	"context"
	"fmt"
	"log/slog"
)

type defaultFailedHandler struct {
	log *slog.Logger
}

func newDefaultFailedHandler(log *slog.Logger) *defaultFailedHandler {
	if log == nil {
		log = slog.Default()
	}
	return &defaultFailedHandler{
		log: log,
	}
}

func (d defaultFailedHandler) Print(ctx context.Context, consumerGroup, topic string, msg []byte, err error) {
	tips := fmt.Sprintf("failed to consumer message to cg: %s, topic: %s, err: %+v", consumerGroup, topic, err)

	attrs := []slog.Attr{slog.String("component", "kafkaconsumer")}

	if ctx != nil {
		if ctx.Err() != nil {
			attrs = append(attrs, slog.String("ctxErr", ctx.Err().Error()))
		}
	}

	d.log.LogAttrs(ctx, slog.LevelError, tips, attrs...)
}
