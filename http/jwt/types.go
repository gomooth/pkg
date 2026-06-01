package jwt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gomooth/pkg/http/httpcontext"
	"github.com/golang-jwt/jwt/v5"
)

// IToken JWT Token 接口，提供令牌创建、配置、刷新和序列化能力
type IToken interface {
	// SetIssuer 设置 token 发行人，默认为 "gomooth/pkg"
	SetIssuer(issuer string) IToken
	// SetDuration 设置 token 过期时长，默认为 24h
	SetDuration(d time.Duration) IToken
	// SetData 设置 token 扩展数据
	SetData(key string, val string) IToken
	// SetSigningMethod 设置签名方法，默认为 HS256
	SetSigningMethod(m jwt.SigningMethod) IToken

	GetUser(fun httpcontext.ToRole) (*httpcontext.User, error)

	// IsStateful 是否为有状态 jwt token
	// 主要用于解决 jwt token 无状态，颁发后不可控。需要业务端注入处理函数
	IsStateful() bool
	// IsExpired 是否过期
	IsExpired() bool

	// Refresh 刷新 token
	Refresh()
	// RefreshNear 自动刷新 token，如果当前时间临近过期时间
	RefreshNear(d time.Duration)

	// ToString 转成 token 字符串
	ToString(ctx context.Context) (string, error)
}

// StatefulStore 状态存储
type StatefulStore interface {
	// Save token 状态存储器
	Save(ctx context.Context, userID uint, token string, expireTs int64) error
	// Check token 状态检查器
	Check(ctx context.Context, userID uint, token string) error
	// Remove 删除指定 token
	Remove(ctx context.Context, userID uint, token string) error
	// Clean 清理用户的所有 token
	Clean(ctx context.Context, userID uint) error
}

// HashFunc converts a token string to a stored representation.
// Default: SHA256 hash. Use IdentityHash to store tokens as-is (for migration).
type HashFunc func(token string) string

// DefaultHashFunc returns the SHA256 hex digest of the token.
func DefaultHashFunc(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// IdentityHash returns the token unchanged. Use during migration from plaintext stores.
func IdentityHash(token string) string {
	return token
}
