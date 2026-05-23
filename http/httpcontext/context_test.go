package httpcontext

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewContext_DefaultTraceID(t *testing.T) {
	ctx, err := NewContext()
	assert.NoError(t, err)
	assert.NotEmpty(t, ctx.TraceID())
}

func TestContext_SetAndGet(t *testing.T) {
	ctx, err := NewContext()
	assert.NoError(t, err)
	ctx.Set("mykey", "myval")

	assert.Equal(t, "myval", ctx.Value("mykey"))
}

func TestContext_SetDoesNotConflictWithStdlib(t *testing.T) {
	ctx, err := NewContext()
	assert.NoError(t, err)

	ctx.Set("timeout", "30s")
	assert.Nil(t, context.Background().Value("timeout"))
	assert.Equal(t, "30s", ctx.Value("timeout"))
}

func TestContext_SetEmptyKeyIgnored(t *testing.T) {
	ctx, err := NewContext()
	assert.NoError(t, err)
	result := ctx.Set("", "val")
	assert.Equal(t, ctx, result)
}

func TestContext_SetUser(t *testing.T) {
	ctx, err := NewContext()
	assert.NoError(t, err)
	user := User{ID: 1, Name: "test"}
	ctx.SetUser(user)

	assert.NotNil(t, ctx.User())
	assert.Equal(t, uint(1), ctx.User().ID)
}

func TestContext_ImplementsContext(t *testing.T) {
	ctx, err := NewContext()
	assert.NoError(t, err)
	var _ context.Context = ctx
}
