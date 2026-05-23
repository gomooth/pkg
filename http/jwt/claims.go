package jwt

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateSecret 生成一个安全的随机密钥，方便测试或开发环境使用。
// 生产环境建议使用配置文件中的固定密钥。
func GenerateSecret(size int) []byte {
	if size < 16 {
		size = 32
	}
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		panic("jwt: failed to generate random secret: " + err.Error())
	}
	return []byte(base64.StdEncoding.EncodeToString(b))
}

type claims struct {
	jwt.RegisteredClaims

	Stateful bool `json:"sf,omitempty"` // 是否有状态。有状态表示 jwt token 被外部存储和判断是否有效

	Account string            `json:"account,omitempty"` // 账号
	UserID  uint              `json:"uid"`               // 用户ID
	Name    string            `json:"name"`              // 姓名
	Roles   []string          `json:"roles"`             // 角色组
	IP      string            `json:"ip,omitempty"`      // 用户登录ID
	Extend  map[string]string `json:"extend,omitempty"`  // 扩展信息
}
