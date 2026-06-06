package cors

import (
	"log/slog"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/gomooth/pkg/http/jwt"
	"github.com/gomooth/pkg/http/restful"
	"github.com/gomooth/utils/sliceutil"
)

type handler struct {
	allowOriginFunc  func(origin string) bool
	allowCredentials *bool // nil 使用默认值 true
	allowMethods     []string
	allowHeaders     []string
	exposeHeaders    []string
	maxAge           time.Duration
}

func New(opts ...Option) gin.HandlerFunc {
	h := &handler{
		allowOriginFunc: nil, // 默认未设置，将输出警告
		allowMethods:    []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		allowHeaders: []string{
			"Origin", "Content-Type", "Accept", "User-Agent", "Cookie", "Authorization",
			"X-Requested-With", "X-Auth-Token", jwt.TokenHeaderKey,
		},
		exposeHeaders: []string{
			"Authorization", "Content-MD5",
			// 分页响应头
			restful.HasMoreHeaderKey, restful.TotalCountHeaderKey, restful.PageInfoHeaderKey, restful.PageLinkHeaderKey,
			// 错误码
			restful.ErrorCodeHeaderKey, restful.ErrorDataHeaderKey,
			// 自动续期 token
			jwt.TokenHeaderKey,
		},
		maxAge: 12 * time.Hour,
	}

	for _, opt := range opts {
		opt(h)
	}

	// 未设置 AllowOriginFunc 时，默认拒绝跨域请求（同源策略）
	if h.allowOriginFunc == nil {
		slog.Warn("cors: no AllowOriginFunc configured, defaulting to same-origin policy. Use WithCORSAllowOriginFunc to specify allowed origins")
		h.allowOriginFunc = func(origin string) bool {
			return false
		}
	}

	return cors.New(h.getCORSConfig())
}

// isWildcardOriginFunc 检查 AllowOriginFunc 是否为通配（对所有 origin 返回 true）
func (ch handler) isWildcardOriginFunc() bool {
	if ch.allowOriginFunc == nil {
		return false
	}
	return ch.allowOriginFunc("https://evil.com") && ch.allowOriginFunc("https://attacker.net")
}

func (ch handler) getCORSConfig() cors.Config {
	credentials := true
	if ch.allowCredentials != nil {
		credentials = *ch.allowCredentials
	}

	if credentials && ch.isWildcardOriginFunc() {
		slog.Warn("cors: AllowCredentials=true with wildcard AllowOriginFunc is a CSRF risk. Consider restricting allowed origins or disabling credentials.")
	}

	return cors.Config{
		AllowOriginFunc:  ch.allowOriginFunc,
		AllowMethods:     sliceutil.Unique(ch.allowMethods),
		AllowHeaders:     sliceutil.Unique(ch.allowHeaders),
		AllowCredentials: credentials,
		ExposeHeaders:    sliceutil.Unique(ch.exposeHeaders),
		MaxAge:           ch.maxAge,
	}
}
