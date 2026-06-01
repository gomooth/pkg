package httpcontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithParent(t *testing.T) {
	t.Run("sets parent context", func(t *testing.T) {
		parent, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx := NewContext(WithParent(parent))
		assert.NotNil(t, ctx)
		// Deadline 应委托给 parent
		_, ok := ctx.Deadline()
		assert.False(t, ok) // Background 和 WithCancel 无 deadline
	})

	t.Run("nil parent uses background", func(t *testing.T) {
		ctx := NewContext(WithParent(nil))
		assert.NotNil(t, ctx)
		// 不会 panic，parent 仍为 Background
		assert.Nil(t, ctx.Err())
	})
}

func TestWithUser(t *testing.T) {
	t.Run("sets user info", func(t *testing.T) {
		user := &User{ID: 1, Account: "test"}
		ctx := NewContext(WithUser(user))
		got := ctx.User()
		assert.Equal(t, uint(1), got.ID)
		assert.Equal(t, "test", got.Account)
	})

	t.Run("returns defensive copy", func(t *testing.T) {
		user := &User{ID: 1, Account: "test"}
		ctx := NewContext(WithUser(user))
		u1 := ctx.User()
		u2 := ctx.User()
		assert.NotSame(t, u1, u2)
	})

	t.Run("nil user", func(t *testing.T) {
		ctx := NewContext(WithUser(nil))
		assert.Nil(t, ctx.User())
	})
}

func TestWithRawRequestBody(t *testing.T) {
	t.Run("stores body in context", func(t *testing.T) {
		body := []byte("test body")
		ctx := NewContext(WithRawRequestBody(body))
		val := ctx.Value(RequestRawBodyDataKey)
		assert.Equal(t, body, val)
	})

	t.Run("nil body", func(t *testing.T) {
		ctx := NewContext(WithRawRequestBody(nil))
		val := ctx.Value(RequestRawBodyDataKey)
		assert.Nil(t, val)
	})
}

func TestWithData(t *testing.T) {
	t.Run("stores key-value pair", func(t *testing.T) {
		ctx := NewContext(WithData("key1", "value1"))
		assert.Equal(t, "value1", ctx.Value("key1"))
	})

	t.Run("multiple data entries", func(t *testing.T) {
		ctx := NewContext(
			WithData("k1", "v1"),
			WithData("k2", 42),
		)
		assert.Equal(t, "v1", ctx.Value("k1"))
		assert.Equal(t, 42, ctx.Value("k2"))
	})

	t.Run("overwrites existing key", func(t *testing.T) {
		ctx := NewContext(
			WithData("key", "old"),
			WithData("key", "new"),
		)
		assert.Equal(t, "new", ctx.Value("key"))
	})
}
