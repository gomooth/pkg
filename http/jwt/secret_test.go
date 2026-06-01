package jwt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/gomooth/xerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------------------------------------------------------------------------
// NewToken / NewStatefulToken empty secret tests (xcode verification)
// ---------------------------------------------------------------------------

func TestNewToken_EmptySecret_XCode(t *testing.T) {
	_, err := NewToken(nil, httpcontext.User{ID: 1, Name: "test"})
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))

	_, err = NewToken([]byte{}, httpcontext.User{ID: 1, Name: "test"})
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
}

func TestNewStatefulToken_EmptySecret_XCode(t *testing.T) {
	_, err := NewStatefulToken(nil, httpcontext.User{ID: 1, Name: "test"}, nil)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
}

// ---------------------------------------------------------------------------
// Basic token creation and parse verification
// ---------------------------------------------------------------------------

func TestNewTokenWithSecret(t *testing.T) {
	secret := []byte("test-secret-key")

	tk, err := NewToken(secret, httpcontext.User{ID: 42, Account: "test-account", Name: "Test User"})
	require.Nil(t, err)

	tokenStr, err := tk.ToString(context.Background())
	require.Nil(t, err)
	assert.NotEmpty(t, tokenStr)

	c, err := parseToken(tokenStr, secret, nil, 0, nil)
	require.Nil(t, err)
	assert.Equal(t, uint(42), c.UserID)
	assert.Equal(t, "test-account", c.Account)
}

// ---------------------------------------------------------------------------
// GenerateSecret
// ---------------------------------------------------------------------------

func TestGenerateSecret(t *testing.T) {
	secret := GenerateSecret(32)
	assert.NotEmpty(t, secret)
	assert.True(t, len(secret) >= 32)

	// Generates different values each time
	secret2 := GenerateSecret(32)
	assert.NotEqual(t, secret, secret2)
}

func TestGenerateSecretMinSize(t *testing.T) {
	secret := GenerateSecret(5)
	assert.NotEmpty(t, secret)
}

// ---------------------------------------------------------------------------
// ParseTokenWithGinAndOption with option-level secret
// ---------------------------------------------------------------------------

func TestOptionSecretWithParseTokenWithGinAndOption(t *testing.T) {
	optSecret := []byte("option-level-secret-32bytes-ok!")
	opt := NewOption(optSecret, testRoleConvert)

	tk, err := NewToken(optSecret, httpcontext.User{ID: 7, Account: "opt-user", Name: "Opt User"})
	require.Nil(t, err)
	tokenStr, err := tk.ToString(context.Background())
	require.Nil(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{Header: make(http.Header)}
	c.Request.Header.Set(TokenHeaderKey, tokenStr)

	_, parsedTk, err := ParseTokenWithGinAndOption(c, opt)
	assert.Nil(t, err)
	assert.NotNil(t, parsedTk)
}

func TestParseTokenWithGinAndOptionNoSecretError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{Header: make(http.Header)}

	_, _, err := ParseTokenWithGinAndOption(c, NewOption(nil, nil))
	assert.NotNil(t, err)

	_, _, err = ParseTokenWithGinAndOption(c, nil)
	assert.NotNil(t, err)
}

// ---------------------------------------------------------------------------
// Legacy secrets with parseToken
// ---------------------------------------------------------------------------

func TestLegacySecrets(t *testing.T) {
	oldSecret := []byte("old-secret-key-32-bytes-long!!!")
	newSecret := []byte("new-secret-key-32-bytes-long!!!")

	// Sign with old secret
	tk, err := NewToken(oldSecret, httpcontext.User{ID: 1, Name: "test"})
	require.Nil(t, err)
	tokenStr, err := tk.ToString(context.Background())
	require.Nil(t, err)

	// Only new secret should fail
	_, err = parseToken(tokenStr, newSecret, nil, 0, nil)
	assert.NotNil(t, err)

	// New secret + legacy rotation should succeed
	c, err := parseToken(tokenStr, newSecret, [][]byte{oldSecret}, 0, nil)
	assert.Nil(t, err)
	assert.Equal(t, uint(1), c.UserID)
}

// ---------------------------------------------------------------------------
// Ensure xerror wrapping is consistent
// ---------------------------------------------------------------------------

func TestParseTokenWithGinAndOption_WrapsError(t *testing.T) {
	opt := NewOption([]byte("s"), testRoleConvert)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{Header: make(http.Header)}
	c.Request.Header.Set(TokenHeaderKey, "invalid")

	_, _, err := ParseTokenWithGinAndOption(c, opt)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTTokenInvalid))
}
