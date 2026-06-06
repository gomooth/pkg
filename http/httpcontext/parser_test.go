package httpcontext

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("non-gin context returns error", func(t *testing.T) {
		_, err := Parse(context.Background())
		assert.Error(t, err)
	})

	t.Run("gin context without httpcontext returns error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		_, err := Parse(c)
		assert.Error(t, err)
	})

	t.Run("gin context with httpcontext returns context", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		stx := NewContext(WithUser(&User{ID: 1}))
		c.Set(ContextKey, stx)
		got, err := Parse(c)
		assert.NoError(t, err)
		assert.Equal(t, uint(1), got.User().ID)
	})

	t.Run("wrong type in context returns error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set(ContextKey, "not an httpcontext")
		_, err := Parse(c)
		assert.Error(t, err)
	})

	t.Run("gin context with nil httpcontext value returns error", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		// Set a nil value with the key — gin.Get returns (nil, true)
		c.Set(ContextKey, nil)
		_, err := Parse(c)
		assert.Error(t, err)
	})
}
