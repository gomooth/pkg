package jwt

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenHeaderKey      = "X-Token"
	TokenQueryStringKey = "token"
)

// ParseTokenWithGin 通过 gin.Context 初始化 token
// 从 gin.Context 优先读取 http header 中的 X-Token 值；如果不存在，则读取 query string 中的 token 值
func ParseTokenWithGin(ctx *gin.Context) (IToken, error) {
	_, tk, err := ParseTokenWithSecret(ctx, jwtSecret)
	return tk, err
}

// ParseTokenWithSecret 解析 token
func ParseTokenWithSecret(ctx *gin.Context, secret []byte) (string, IToken, error) {
	tokenStr := getTokenStr(ctx)

	c, err := parseToken(tokenStr, secret)
	if nil != err {
		return "", nil, err
	}

	// 通用参数
	c.IP = ctx.ClientIP()

	tk := newTokenWith(c).SetSecret(secret)

	return tokenStr, tk, nil
}

// getTokenStr 获取请求中的 token 字符串
// 优先读取 http header 中的 X-Token 值；如果不存在，则读取 query string 中的 token 值
func getTokenStr(ctx *gin.Context) string {
	tokenStr := strings.TrimSpace(ctx.GetHeader(TokenHeaderKey))
	if len(tokenStr) == 0 {
		tokenStr, _ = ctx.GetQuery(TokenQueryStringKey)
	}

	return strings.TrimSpace(tokenStr)
}

func parseToken(token string, secret []byte) (*claims, error) {
	if len(secret) == 0 {
		secret = jwtSecret
	}

	tokenClaims, err := jwt.ParseWithClaims(token, &claims{}, func(token *jwt.Token) (interface{}, error) {
		// Don't forget to validate the alg is what you expect:
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		return secret, nil
	})

	if tokenClaims == nil {
		return nil, err
	}

	if c, ok := tokenClaims.Claims.(*claims); ok && tokenClaims.Valid {
		return c, nil
	}

	return nil, err
}
