package jwt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/http/httpcontext"
	pkgjwt "github.com/gomooth/pkg/http/jwt"
	"github.com/gomooth/xerror"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newTestGinContext creates a minimal gin.Context with an empty request for testing.
func newMiddlewareTestGinContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
	}
	return c
}

// mockRoleConvert is a simple ToRole function for testing
func mockRoleConvert(role string) (httpcontext.IRole, error) {
	return mockRole(role), nil
}

// mockRole implements IRole for testing
type mockRole string

func (r mockRole) String() string { return string(r) }

// TestHandler_Handle_NilOption tests that Handle with nil option returns ErrJWTTokenInvalid
func TestHandler_Handle_NilOption(t *testing.T) {
	c := newMiddlewareTestGinContext()
	h := NewHandler(c, nil)

	err := h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// TestHandler_Handle_InvalidToken tests that Handle with an invalid token string returns ErrJWTTokenInvalid
func TestHandler_Handle_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, "invalid-token-string")

	opt := pkgjwt.NewOption(secret, mockRoleConvert)
	h := NewHandler(c, opt)

	err := h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// TestHandler_Handle_EmptyToken tests that Handle with no token in header returns ErrJWTTokenInvalid
func TestHandler_Handle_EmptyToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	c := newMiddlewareTestGinContext()

	opt := pkgjwt.NewOption(secret, mockRoleConvert)
	h := NewHandler(c, opt)

	err := h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// TestHandler_Handle_ValidToken tests that Handle with a valid token succeeds
func TestHandler_Handle_ValidToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
		Roles:   nil,
	}

	tk, err := pkgjwt.NewTokenBuilder(secret, user).Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert)
	h := NewHandler(c, opt)

	err = h.Handle()
	assert.Nil(t, err)
}

// TestHandler_Handle_ValidToken_WithOptionStyle tests that Handle with NewOption + With* style succeeds
func TestHandler_Handle_ValidToken_WithOptionStyle(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
		Roles:   nil,
	}

	tk, err := pkgjwt.NewTokenBuilder(secret, user).Build()
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert,
		pkgjwt.WithSilentMode(false),
	)
	h := NewHandler(c, opt)

	err = h.Handle()
	assert.Nil(t, err)
}

// TestStatefulHandler_Handle_NilOption tests that stateful handler with nil option returns ErrJWTTokenInvalid
func TestStatefulHandler_Handle_NilOption(t *testing.T) {
	c := newMiddlewareTestGinContext()
	h := NewStatefulHandler(c, nil, nil)

	err := h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// TestStatefulHandler_Handle_NilStoreWithNonSkipCheck tests stateful handler with nil store but not skip check
func TestStatefulHandler_Handle_NilStoreWithNonSkipCheck(t *testing.T) {
	c := newMiddlewareTestGinContext()

	h := NewStatefulHandler(c, nil, nil)
	err := h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// TestStatefulHandler_Handle_InvalidToken tests that stateful handler with invalid token returns ErrJWTTokenInvalid
func TestStatefulHandler_Handle_InvalidToken(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, "invalid-token-string")

	opt := pkgjwt.NewOption(secret, mockRoleConvert)

	h := NewStatefulHandler(c, opt, nil)

	err := h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// TestHandler_Handle_NonStatefulTokenOnStatefulHandler tests that a non-stateful token
// used with stateful handler returns ErrJWTTokenInvalid
func TestHandler_Handle_NonStatefulTokenOnStatefulHandler(t *testing.T) {
	secret := []byte("test-secret-that-is-long-enough-32b!")

	user := httpcontext.User{
		ID:      1,
		Account: "test-account",
		Name:    "Test User",
	}

	tk, err := pkgjwt.NewTokenBuilder(secret, user).Build() // non-stateful token
	if err != nil {
		t.Fatalf("cannot create token: %v", err)
	}
	tokenStr, err := tk.ToString(context.Background())
	if err != nil {
		t.Fatalf("cannot generate token: %v", err)
	}

	c := newMiddlewareTestGinContext()
	c.Request.Header.Set(pkgjwt.TokenHeaderKey, tokenStr)

	opt := pkgjwt.NewOption(secret, mockRoleConvert)

	store := &mockStatefulStore{}

	h := NewStatefulHandler(c, opt, store)

	err = h.Handle()
	assert.NotNil(t, err)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}

// mockStatefulStore is a mock implementation of StatefulStore for testing
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
