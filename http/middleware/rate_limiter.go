package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/gomooth/pkg/http/middleware/internal/limit"

	"github.com/ulule/limiter/v3"
)

// RateLimit 限速
func RateLimit(key string, limiter *limiter.Limiter) gin.HandlerFunc {
	return limit.RateLimiter(key, limiter)
}

// IPRateLimit IP限速
func IPRateLimit(limiter *limiter.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limit.RateLimiter(ip, limiter)(c)
	}
}
