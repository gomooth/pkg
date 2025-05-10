package jwt

import (
	"time"

	"github.com/gomooth/pkg/http/httpcontext"
)

type IToken interface {
	// SetIssuer 设置 token 发行人，默认为 "go-pkg"
	SetIssuer(issuer string) IToken
	// SetDuration 设置 token 过期时长，默认为 24h
	SetDuration(d time.Duration) IToken
	// SetSecret 设置 token 加密密钥，默认 "go-pkg.JwtSecret"
	SetSecret(secret []byte) IToken
	// SetData 设置 token 扩展数据
	SetData(key string, val string) IToken

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
	ToString() (string, error)
}

// StatefulStore 状态存储
type StatefulStore interface {
	// Save token 状态存储器
	Save(userID uint, token string, expireTs int64) error
	// Check token 状态检查器
	Check(userID uint, token string) error
	// Remove 删除指定 token
	Remove(userID uint, token string) error
	// Clean 清理用户的所有 token
	Clean(userID uint) error
}
