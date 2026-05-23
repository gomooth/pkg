package jwt

import (
	"context"
	"testing"
	"time"

	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/stretchr/testify/assert"
)

func TestToken(t *testing.T) {
	user := httpcontext.User{
		ID:      1,
		Account: "account",
		Name:    "My Name",
		Roles:   nil,
		IP:      "127.0.0.1",
		Extend: map[string]string{
			"a": "124",
		},
	}

	secret := []byte("test-secret-key")

	// 创建 token 并签发
	tk, err := NewToken(secret, user)
	assert.Nil(t, err)

	tokenStr, err := tk.ToString(context.Background())
	assert.Nil(t, err)
	assert.NotEmpty(t, tokenStr)

	// 解析签发的 token
	c, err := parseToken(tokenStr, secret, nil, 0, nil)
	assert.Nil(t, err)
	assert.NotNil(t, c)

	// 验证 claims 内容
	assert.Equal(t, "gomooth/pkg", c.Issuer)
	assert.Equal(t, user.ID, c.UserID)
	assert.Equal(t, user.Account, c.Account)
	assert.Equal(t, user.Name, c.Name)
	assert.Empty(t, c.IP) // 默认不包含 IP
	assert.Equal(t, "124", c.Extend["a"])
	assert.False(t, c.Stateful)

	// 从解析的 claims 构建新 token，重新签发并验证可解析
	ntk := newTokenWith(c)
	ntk.secret = secret
	newTokenStr, err := ntk.ToString(context.Background())
	assert.Nil(t, err)
	assert.NotEmpty(t, newTokenStr)

	// 验证新 token 可以正确解析
	nc, err := parseToken(newTokenStr, secret, nil, 0, nil)
	assert.Nil(t, err)
	assert.Equal(t, user.ID, nc.UserID)
	assert.Equal(t, user.Account, nc.Account)
}

func TestTokenExpired(t *testing.T) {
	secret := []byte("test-secret-key")

	tk, err := NewToken(secret, httpcontext.User{ID: 1, Name: "test"})
	assert.Nil(t, err)
	assert.False(t, tk.IsExpired())

	// 设置极短过期时间
	tk.SetDuration(-1 * time.Second)
	assert.True(t, tk.IsExpired())
}

func TestTokenRefresh(t *testing.T) {
	secret := []byte("test-secret-key")

	tk, err := NewToken(secret, httpcontext.User{ID: 1, Name: "test"})
	assert.Nil(t, err)

	tk.SetDuration(1 * time.Second)

	// 刷新后不应过期
	tk.Refresh()
	assert.False(t, tk.IsExpired())
}

func TestTokenStateful(t *testing.T) {
	tk, err := NewToken([]byte("test-secret-key"), httpcontext.User{ID: 1, Name: "test"})
	assert.Nil(t, err)
	assert.False(t, tk.IsStateful())

	stk, err := NewStatefulToken([]byte("test-secret-key"), httpcontext.User{ID: 1, Name: "test"}, nil)
	assert.Nil(t, err)
	assert.True(t, stk.IsStateful())
}

func TestTokenSetData(t *testing.T) {
	secret := []byte("test-secret-key")

	tk, err := NewToken(secret, httpcontext.User{ID: 1, Name: "test"})
	assert.Nil(t, err)

	tk.SetData("key1", "value1")
	tk.SetData("key2", "value2")

	tokenStr, err := tk.ToString(context.Background())
	assert.Nil(t, err)

	c, err := parseToken(tokenStr, secret, nil, 0, nil)
	assert.Nil(t, err)
	assert.Equal(t, "value1", c.Extend["key1"])
	assert.Equal(t, "value2", c.Extend["key2"])
}

func TestTokenWithCustomIssuer(t *testing.T) {
	secret := []byte("test-secret-key")

	tk, err := NewToken(secret, httpcontext.User{ID: 1, Name: "test"})
	assert.Nil(t, err)

	tk.SetIssuer("custom-issuer")

	tokenStr, err := tk.ToString(context.Background())
	assert.Nil(t, err)

	c, err := parseToken(tokenStr, secret, nil, 0, nil)
	assert.Nil(t, err)
	assert.Equal(t, "custom-issuer", c.Issuer)
}
