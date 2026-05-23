package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/gomooth/pkg/http/middleware/internal/limit"

	"github.com/ulule/limiter/v3"
)

// RateLimit 限速
func RateLimit(key string, limiter *limiter.Limiter) gin.HandlerFunc {
	return limit.RateLimiter(func(*gin.Context) string {
		return key
	}, limiter)
}

// DynamicRateLimit 动态 key 限速，key 从请求上下文中提取
// 适用于按用户、按租户等维度的限流
func DynamicRateLimit(keyFn func(*gin.Context) string, limiter *limiter.Limiter) gin.HandlerFunc {
	return limit.RateLimiter(keyFn, limiter)
}

// IPRateLimit IP限速
func IPRateLimit(limiter *limiter.Limiter) gin.HandlerFunc {
	return limit.RateLimiter(func(c *gin.Context) string {
		return c.ClientIP()
	}, limiter)
}
