package jwt

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gomooth/pkg/framework/xcode"
	"github.com/gomooth/xerror"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenHeaderKey      = "X-Token"
	TokenQueryStringKey = "token"
)

// ParseTokenWithGinAndOption 通过 gin.Context 和选项初始化 token
func ParseTokenWithGinAndOption(ctx *gin.Context, opt *Option) (string, IToken, error) {
	if opt == nil || len(opt.Secret()) == 0 {
		return "", nil, xerror.NewXCode(xcode.ErrJWTSecretNotSet, "jwt: secret not set")
	}

	tokenStr := getTokenStrWithPaths(ctx, opt)

	c, err := parseToken(tokenStr, opt.Secret(), opt.LegacySecrets(), opt.Leeway(), opt.SigningMethods())
	if err != nil {
		return "", nil, xerror.WrapWithXCode(err, xcode.ErrJWTTokenInvalid)
	}

	c.IP = ctx.ClientIP()
	tk := newTokenWith(c)
	tk.secret = opt.Secret()
	return tokenStr, tk, nil
}

// ParseTokenWithSecret 解析 token
func ParseTokenWithSecret(ctx *gin.Context, secret []byte, legacySecrets ...[]byte) (string, IToken, error) {
	tokenStr := getTokenStr(ctx, false)

	c, err := parseToken(tokenStr, secret, legacySecrets, 0, nil)
	if err != nil {
		return "", nil, err
	}

	// 通用参数
	c.IP = ctx.ClientIP()

	tk := newTokenWith(c)
	tk.secret = secret

	return tokenStr, tk, nil
}

// getTokenStr 获取请求中的 token 字符串
func getTokenStr(ctx *gin.Context, allowQueryString bool) string {
	tokenStr := strings.TrimSpace(ctx.GetHeader(TokenHeaderKey))
	if len(tokenStr) == 0 && allowQueryString {
		tokenStr, _ = ctx.GetQuery(TokenQueryStringKey)
	}

	return strings.TrimSpace(tokenStr)
}

// getTokenStrWithPaths 获取请求中的 token 字符串，支持路径白名单
func getTokenStrWithPaths(ctx *gin.Context, opt *Option) string {
	tokenStr := strings.TrimSpace(ctx.GetHeader(TokenHeaderKey))
	if len(tokenStr) == 0 && opt != nil && opt.AllowQueryStringToken() {
		if paths := opt.QueryStringTokenPaths(); len(paths) > 0 {
			// 路径白名单模式：仅匹配的路径才从 query string 读取
			path := ctx.Request.URL.Path
			for _, p := range paths {
				if path == p {
					tokenStr, _ = ctx.GetQuery(TokenQueryStringKey)
					break
				}
			}
		} else {
			// 全局开放模式：打印安全警告
			slog.Warn("jwt: token received via query string without path restriction, consider using WithAllowQueryStringToken with paths to limit",
				slog.String("component", "jwt"),
				slog.String("path", ctx.Request.URL.Path),
			)
			tokenStr, _ = ctx.GetQuery(TokenQueryStringKey)
		}
	}

	return strings.TrimSpace(tokenStr)
}

func parseToken(token string, secret []byte, legacySecrets [][]byte, leeway time.Duration, signingMethods []string) (*claims, error) {
	if len(secret) == 0 {
		return nil, xerror.NewXCode(xcode.ErrJWTSecretNotSet, "jwt: secret not set")
	}

	// 先尝试主密钥
	c, err := parseTokenWithSecret(token, secret, leeway, signingMethods)
	if err == nil {
		return c, nil
	}

	// 主密钥失败，依次尝试旧版密钥
	for _, legacySecret := range legacySecrets {
		if c, legacyErr := parseTokenWithSecret(token, legacySecret, leeway, signingMethods); legacyErr == nil {
			return c, nil
		}
	}

	return nil, err
}

// defaultSigningMethods 默认允许的 HMAC 签名算法
var defaultSigningMethods = []string{"HS256", "HS384", "HS512"}

// parseTokenWithSecret 使用指定密钥解析 token
func parseTokenWithSecret(token string, secret []byte, leeway time.Duration, signingMethods []string) (*claims, error) {
	methods := signingMethods
	if len(methods) == 0 {
		methods = defaultSigningMethods
	}
	parserOpts := []jwt.ParserOption{
		jwt.WithValidMethods(methods),
	}
	if leeway > 0 {
		parserOpts = append(parserOpts, jwt.WithLeeway(leeway))
	}

	tokenClaims, err := jwt.ParseWithClaims(token, &claims{}, func(token *jwt.Token) (any, error) {
		return secret, nil
	}, parserOpts...)

	if tokenClaims == nil {
		return nil, err
	}

	if c, ok := tokenClaims.Claims.(*claims); ok && tokenClaims.Valid {
		return c, nil
	}

	return nil, err
}
