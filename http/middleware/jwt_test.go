package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/httpcontext"
	pkgjwt "github.com/gomooth/pkg/http/jwt"
	"github.com/stretchr/testify/assert"
)

func mockToRole(role string) (httpcontext.IRole, error) {
	return testRole(role), nil
}

func TestJWTWith_NoToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	r := gin.New()
	r.Use(JWTWith(secret, mockToRole))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTWith_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	r := gin.New()
	r.Use(JWTWith(secret, mockToRole))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(pkgjwt.TokenHeaderKey, "invalid-token")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTWith_ValidToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
		Roles:   []httpcontext.IRole{testRole("admin")},
	}

	tk, err := pkgjwt.NewTokenBuilder(secret, user).Build()
	assert.NoError(t, err)
	tokenStr, err := tk.ToString(context.Background())
	assert.NoError(t, err)

	r := gin.New()
	r.Use(JWTWith(secret, mockToRole))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJWTWith_SilentMode(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	handlerCalled := false
	r := gin.New()
	r.Use(JWTWith(secret, mockToRole, pkgjwt.WithSilentMode(true)))
	r.GET("/test", func(c *gin.Context) {
		handlerCalled = true
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	// 静默模式下没有 token，c.Abort() 中断后续 handler 但不写入错误
	// handler 不应被调用
	assert.False(t, handlerCalled, "handler should not be called in silent mode with no token")
}

func TestJWTStatefulWithout_ValidToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	// 无状态 token（不设置 StatefulStore），ToString 应成功
	tk, err := pkgjwt.NewTokenBuilder(secret, user).Build()
	assert.NoError(t, err)
	_, err = tk.ToString(context.Background())
	assert.NoError(t, err)

	// 测试有 stateful token 但使用 JWTStatefulWithout（跳过状态校验）的场景
	store := &mockStatefulStore{}
	tk2, err := pkgjwt.NewTokenBuilder(secret, user).WithStatefulStore(store).Build()
	assert.NoError(t, err)
	tokenStr, err := tk2.ToString(context.Background())
	assert.NoError(t, err)

	r := gin.New()
	r.Use(JWTStatefulWithout(secret, mockToRole))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestJWTStatefulWithout_NoToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	r := gin.New()
	r.Use(JWTStatefulWithout(secret, mockToRole))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestJWTStatefulWith_NoToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")
	store := &mockStatefulStore{}

	r := gin.New()
	r.Use(JWTStatefulWith(secret, mockToRole, store))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// mockStatefulStore implements jwt.StatefulStore for testing
type mockStatefulStore struct{}

func (m *mockStatefulStore) Save(_ context.Context, _ uint, _ string, _ int64) error {
	return nil
}

func (m *mockStatefulStore) Check(_ context.Context, _ uint, _ string) error {
	return nil
}

func (m *mockStatefulStore) Remove(_ context.Context, _ uint, _ string) error {
	return nil
}

func (m *mockStatefulStore) Clean(_ context.Context, _ uint) error {
	return nil
}
