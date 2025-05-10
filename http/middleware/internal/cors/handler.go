package cors

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/gomooth/pkg/http/jwt"
	"github.com/gomooth/pkg/http/restful"
	"github.com/gomooth/utils/sliceutil"
)

type handler struct {
	allowOriginFunc func(origin string) bool
	allowMethods    []string
	allowHeaders    []string
	exposeHeaders   []string
	maxAge          time.Duration
}

func New(opts ...Option) gin.HandlerFunc {
	h := &handler{
		allowOriginFunc: func(origin string) bool {
			//return origin == "https://xxxx.com"
			return true
		},
		allowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
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

	return cors.New(h.getCORSConfig())
}

func (ch handler) getCORSConfig() cors.Config {
	return cors.Config{
		AllowOriginFunc:  ch.allowOriginFunc,
		AllowMethods:     sliceutil.Unique(ch.allowMethods...),
		AllowHeaders:     sliceutil.Unique(ch.allowHeaders...),
		AllowCredentials: true,
		ExposeHeaders:    sliceutil.Unique(ch.exposeHeaders...),
		MaxAge:           ch.maxAge,
	}
}
