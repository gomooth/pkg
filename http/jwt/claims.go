package jwt

import (
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte("gomooth-pkg.JwtSecret")

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
