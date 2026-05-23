package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

type SessionOption struct {
	Path   string
	Domain string
	MaxAge time.Duration

	// Secure cookie 是否仅通过 HTTPS 传输。nil 使用默认值 false
	Secure *bool
	// HttpOnly cookie 是否禁止 JavaScript 访问。nil 使用默认值 true
	HttpOnly *bool
	// SameSite cookie 的 SameSite 策略。0 使用默认值 Lax
	SameSite http.SameSite
}

func resolveSessionDefaults(opt SessionOption) (secure bool, httpOnly bool, sameSite http.SameSite) {
	secure = false
	if opt.Secure != nil {
		secure = *opt.Secure
	}

	httpOnly = true
	if opt.HttpOnly != nil {
		httpOnly = *opt.HttpOnly
	} else {
		slog.Warn("session: HttpOnly not set, defaulting to true for security")
	}

	sameSite = http.SameSiteLaxMode
	if opt.SameSite != 0 {
		sameSite = opt.SameSite
	} else {
		slog.Warn("session: SameSite not set, defaulting to Lax for security")
	}

	return
}

// Session 校验 session
// keyPairs cookie 键名
// secret cookie 存储加密密钥（必填，为空时 panic）
func Session(keyPairs, secret string, opt SessionOption) gin.HandlerFunc {
	if len(secret) == 0 {
		panic("session: secret is required. Provide a non-empty secret or use SessionWithSecretFromEnv()")
	}

	secure, httpOnly, sameSite := resolveSessionDefaults(opt)

	var store sessions.Store
	store = cookie.NewStore([]byte(secret))
	store.Options(sessions.Options{
		Path:     opt.Path,
		Domain:   opt.Domain,
		MaxAge:   int(opt.MaxAge.Seconds()),
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: sameSite,
	})

	return sessions.Sessions(keyPairs, store)
}

// SessionWithStore 校验 session
// keyPairs cookie 键名
func SessionWithStore(keyPairs string, store sessions.Store, opt SessionOption) gin.HandlerFunc {
	secure, httpOnly, sameSite := resolveSessionDefaults(opt)

	store.Options(sessions.Options{
		Path:     opt.Path,
		Domain:   opt.Domain,
		MaxAge:   int(opt.MaxAge.Seconds()),
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: sameSite,
	})

	return sessions.Sessions(keyPairs, store)
}

// SessionWithSecretFromEnv 从环境变量读取 session 密钥
func SessionWithSecretFromEnv(keyPairs, envKey string, opt SessionOption) gin.HandlerFunc {
	secret := os.Getenv(envKey)
	if len(secret) == 0 {
		panic(fmt.Sprintf("session: environment variable %s is not set or empty", envKey))
	}
	return Session(keyPairs, secret, opt)
}
