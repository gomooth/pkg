package internal

import "context"

func WithHandler(handler func(ctx context.Context, topic string, msg []byte) error) func(*groupHandler) {
	return func(h *groupHandler) {
		h.handler = handler
	}
}

func WithFailedHandler(handler func(ctx context.Context, consumerGroup, topic string, msg []byte, err error)) func(*groupHandler) {
	return func(h *groupHandler) {
		h.failedHandler = handler
	}
}
