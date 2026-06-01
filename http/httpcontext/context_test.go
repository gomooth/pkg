package httpcontext

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// testRole 测试用 IRole 实现
type testRole string

func (r testRole) String() string { return string(r) }

var (
	ToRoleAdmin = testRole("admin")
	ToRoleUser  = testRole("user")
)

func TestNewContext_DefaultBehavior(t *testing.T) {
	ctx := NewContext()
	assert.NotNil(t, ctx)
	assert.Nil(t, ctx.User())
	assert.Nil(t, ctx.Err())
}

func TestContext_SetAndGet(t *testing.T) {
	ctx := NewContext()
	ctx.Set("mykey", "myval")

	assert.Equal(t, "myval", ctx.Value("mykey"))
}

func TestContext_SetDoesNotConflictWithStdlib(t *testing.T) {
	ctx := NewContext()

	ctx.Set("timeout", "30s")
	assert.Nil(t, context.Background().Value("timeout"))
	assert.Equal(t, "30s", ctx.Value("timeout"))
}

func TestContext_SetEmptyKeyIgnored(t *testing.T) {
	ctx := NewContext()
	result := ctx.Set("", "val")
	assert.Equal(t, ctx, result)
}

func TestContext_SetUser(t *testing.T) {
	ctx := NewContext()
	user := User{ID: 1, Name: "test"}
	ctx.SetUser(user)

	assert.NotNil(t, ctx.User())
	assert.Equal(t, uint(1), ctx.User().ID)
}

func TestContext_SetUser_DefensiveCopy(t *testing.T) {
	ctx := NewContext()
	user := User{ID: 1, Name: "test"}
	ctx.SetUser(user)

	// 修改返回值不应影响内部状态
	returned := ctx.User()
	returned.ID = 999
	assert.Equal(t, uint(1), ctx.User().ID)
}

func TestContext_IsRole(t *testing.T) {
	t.Run("no user returns false", func(t *testing.T) {
		ctx := NewContext()
		assert.False(t, ctx.IsRole(ToRoleAdmin))
	})

	t.Run("user with matching role returns true", func(t *testing.T) {
		ctx := NewContext(WithUser(&User{
			ID:    1,
			Roles: []IRole{ToRoleAdmin},
		}))
		assert.True(t, ctx.IsRole(ToRoleAdmin))
	})

	t.Run("user with different role returns false", func(t *testing.T) {
		ctx := NewContext(WithUser(&User{
			ID:    1,
			Roles: []IRole{ToRoleUser},
		}))
		assert.False(t, ctx.IsRole(ToRoleAdmin))
	})
}

func TestContext_ImplementsContext(t *testing.T) {
	ctx := NewContext()
	var _ context.Context = ctx
}

func TestContext_Deadline(t *testing.T) {
	t.Run("background context has no deadline", func(t *testing.T) {
		ctx := NewContext()
		_, ok := ctx.Deadline()
		assert.False(t, ok)
	})

	t.Run("with deadline parent delegates", func(t *testing.T) {
		deadline := time.Now().Add(time.Hour)
		parent, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		ctx := NewContext(WithParent(parent))
		dl, ok := ctx.Deadline()
		assert.True(t, ok)
		assert.Equal(t, deadline, dl)
	})
}

func TestContext_Done(t *testing.T) {
	t.Run("background context has nil Done channel", func(t *testing.T) {
		ctx := NewContext()
		assert.Nil(t, ctx.Done())
	})

	t.Run("cancelled parent has non-nil Done channel", func(t *testing.T) {
		parent, cancel := context.WithCancel(context.Background())
		ctx := NewContext(WithParent(parent))
		assert.NotNil(t, ctx.Done())
		cancel()
		<-ctx.Done()
	})
}

func TestContext_Err(t *testing.T) {
	t.Run("background context has no error", func(t *testing.T) {
		ctx := NewContext()
		assert.Nil(t, ctx.Err())
	})

	t.Run("cancelled context returns error", func(t *testing.T) {
		parent, cancel := context.WithCancel(context.Background())
		ctx := NewContext(WithParent(parent))
		cancel()
		assert.Error(t, ctx.Err())
	})
}

func TestContext_Value(t *testing.T) {
	t.Run("string key lookup", func(t *testing.T) {
		ctx := NewContext(WithData("mykey", "myval"))
		assert.Equal(t, "myval", ctx.Value("mykey"))
	})

	t.Run("ctxKey lookup", func(t *testing.T) {
		ctx := NewContext(WithData("mykey", "myval"))
		assert.Equal(t, "myval", ctx.Value(ctxKey("mykey")))
	})

	t.Run("non-string non-ctxKey key falls through to parent", func(t *testing.T) {
		type customKey int
		parent := context.WithValue(context.Background(), customKey(1), "from-parent")
		ctx := NewContext(WithParent(parent))
		assert.Equal(t, "from-parent", ctx.Value(customKey(1)))
	})

	t.Run("missing key returns nil", func(t *testing.T) {
		ctx := NewContext()
		assert.Nil(t, ctx.Value("nonexistent"))
	})
}

func TestContext_StorageTo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("stores context in gin.Context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		ctx := NewContext(WithUser(&User{ID: 42}))

		result := ctx.StorageTo(c)
		assert.True(t, result)

		val, exists := c.Get(ContextKey)
		assert.True(t, exists)
		stored, ok := val.(IHttpContext)
		assert.True(t, ok)
		assert.Equal(t, uint(42), stored.User().ID)
	})
}
