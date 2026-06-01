package middleware

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/middleware/internal/logger"
)

const httpLoggerDisabledKey = "gomooth/pkg/middleware:httpLoggerDisabled"

// HttpLoggerOption HTTP 日志中间件的配置选项
type HttpLoggerOption struct {
	Logger          *slog.Logger
	OnlyError       bool     // 仅发生错误时，打印日志；否则，打印所有请求
	RedactEnabled   *bool    // 是否启用敏感字段脱敏，nil 或 true 时开启，显式设为 false 关闭
	SensitiveFields []string // 敏感字段名，日志中会被替换为 ***；为 nil 时使用默认值
}

// HttpLogger http 日志中间件
//
// usage:
//
//	  r.Use(middleware.HttpLogger(middleware.HttpLoggerOption{
//		 	Logger:    global.Log,
//		 	OnlyError: global.Config.Log.HttpLogOnlyError,
//		 }))
func HttpLogger(opt HttpLoggerOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		redact := opt.RedactEnabled == nil || *opt.RedactEnabled
		l := logger.New(c, redact, opt.SensitiveFields)

		c.Next()

		// 检查是否有其他日志处理器已禁用此中间件的日志输出
		if _, disabled := c.Get(httpLoggerDisabledKey); disabled {
			return
		}

		needLog := true
		errors := c.Errors.ByType(gin.ErrorTypeAny)
		if len(errors) == 0 && opt.OnlyError {
			needLog = false
		}

		if needLog {
			opt.Logger.Info(l.String())
		}
	}
}

// DisableHttpLogger 在 gin.Context 中标记禁用 HttpLogger 的日志输出。
// 其他日志中间件（如 HttpPrinter）应调用此函数代替字符串匹配。
func DisableHttpLogger(c *gin.Context) {
	c.Set(httpLoggerDisabledKey, true)
}
