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
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewToken_EmptySecret(t *testing.T) {
	_, err := NewToken(nil, httpcontext.User{ID: 1, Name: "test"})
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))

	_, err = NewToken([]byte{}, httpcontext.User{ID: 1, Name: "test"})
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
}

func TestNewStatefulToken_EmptySecret(t *testing.T) {
	_, err := NewStatefulToken(nil, httpcontext.User{ID: 1, Name: "test"}, nil)
	assert.True(t, xerror.IsXCode(err, xcode.ErrJWTSecretNotSet))
}

func TestNewTokenWithSecret(t *testing.T) {
	secret := []byte("test-secret-key")

	tk, err := NewToken(secret, httpcontext.User{ID: 42, Account: "test-account", Name: "Test User"})
	assert.Nil(t, err)

	tokenStr, err := tk.ToString(context.Background())
	assert.Nil(t, err)
	assert.NotEmpty(t, tokenStr)

	c, err := parseToken(tokenStr, secret, nil, 0, nil)
	assert.Nil(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, uint(42), c.UserID)
	assert.Equal(t, "test-account", c.Account)
}

func TestGenerateSecret(t *testing.T) {
	secret := GenerateSecret(32)
	assert.NotEmpty(t, secret)
	assert.True(t, len(secret) >= 32)

	// 生成两次应不同
	secret2 := GenerateSecret(32)
	assert.NotEqual(t, secret, secret2)
}

func TestGenerateSecretMinSize(t *testing.T) {
	secret := GenerateSecret(5)
	assert.NotEmpty(t, secret)
}

func TestOptionSecretWithParseTokenWithGinAndOption(t *testing.T) {
	optSecret := []byte("option-level-secret-32bytes-ok!")
	opt := NewOption(optSecret, func(role string) (httpcontext.IRole, error) { return mockTestRole(role), nil })

	user := httpcontext.User{ID: 7, Account: "opt-user", Name: "Opt User"}
	tk, err := NewToken(optSecret, user)
	assert.Nil(t, err)

	tokenStr, err := tk.ToString(context.Background())
	assert.Nil(t, err)

	c := newSecretTestGinContext()
	c.Request.Header.Set(TokenHeaderKey, tokenStr)

	_, parsedTk, err := ParseTokenWithGinAndOption(c, opt)
	assert.Nil(t, err)
	assert.NotNil(t, parsedTk)
}

func TestParseTokenWithGinAndOptionNoSecretError(t *testing.T) {
	c := newSecretTestGinContext()

	_, _, err := ParseTokenWithGinAndOption(c, NewOption(nil, nil))
	assert.NotNil(t, err)

	_, _, err = ParseTokenWithGinAndOption(c, nil)
	assert.NotNil(t, err)
}

func TestLegacySecrets(t *testing.T) {
	oldSecret := []byte("old-secret-key-32-bytes-long!!!")
	newSecret := []byte("new-secret-key-32-bytes-long!!!")

	// 用旧密钥签发 token
	tk, err := NewToken(oldSecret, httpcontext.User{ID: 1, Name: "test"})
	assert.Nil(t, err)

	tokenStr, err := tk.ToString(context.Background())
	assert.Nil(t, err)

	// 只用新密钥解析应失败
	_, err = parseToken(tokenStr, newSecret, nil, 0, nil)
	assert.NotNil(t, err)

	// 新密钥 + 旧密钥轮换应成功
	c, err := parseToken(tokenStr, newSecret, [][]byte{oldSecret}, 0, nil)
	assert.Nil(t, err)
	assert.Equal(t, uint(1), c.UserID)
}

func newSecretTestGinContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{Header: make(http.Header)}
	return c
}
