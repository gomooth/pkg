package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/middleware/internal/logger"
)

// HttpPrinter 打印 http 信息中间件；展示 request / response 等信息
// 用于调试场景，不进行敏感字段脱敏
//
// usage:
//
//	r.Use(middleware.HttpContext(global.Log))
//
//	router.Any("/endpoint", middleware.HttpPrinter(global.Log), ping.Controller{}.Endpoint)
func HttpPrinter(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		DisableHttpLogger(c)

		l := logger.New(c, false, nil)

		c.Next()

		log.Info(l.String())
	}
}
