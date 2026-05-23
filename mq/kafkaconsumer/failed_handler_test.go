package kafkaconsumer

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultFailedHandler_NilLogger(t *testing.T) {
	h := newDefaultFailedHandler(nil)
	assert.NotNil(t, h.log, "nil logger should fall back to slog.Default()")
	assert.NotPanics(t, func() {
		h.Print(context.Background(), "test-group", "test-topic", []byte("msg"), assert.AnError)
	})
}

func TestDefaultFailedHandler_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, nil))
	h := newDefaultFailedHandler(l)

	h.Print(context.Background(), "my-group", "my-topic", []byte("payload"), assert.AnError)

	output := buf.String()
	assert.Contains(t, output, "my-group")
	assert.Contains(t, output, "my-topic")
	assert.Contains(t, output, "kafkaconsumer")
}
