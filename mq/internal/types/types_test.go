package types

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- FuncHandler.Handle ---

func TestFuncHandler_Handle_Success(t *testing.T) {
	called := false
	handler := FuncHandler(func(ctx context.Context, msg Message) error {
		called = true
		assert.Equal(t, "q", msg.Queue)
		return nil
	})

	err := handler.Handle(context.Background(), NewRedisMessage("q", []byte("data")))
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestFuncHandler_Handle_Error(t *testing.T) {
	expectedErr := errors.New("handler failed")
	handler := FuncHandler(func(ctx context.Context, msg Message) error {
		return expectedErr
	})

	err := handler.Handle(context.Background(), NewKafkaMessage("g", "t", nil))
	assert.ErrorIs(t, err, expectedErr)
}

func TestFuncHandler_InterfaceCompliance(t *testing.T) {
	// FuncHandler 应实现 IHandler 接口
	var _ IHandler = FuncHandler(func(ctx context.Context, msg Message) error {
		return nil
	})
}
