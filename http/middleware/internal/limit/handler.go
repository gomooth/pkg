package limit

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ulule/limiter/v3"
)

// RateLimiter 创建限速器中间件
func RateLimiter(key string, limit *limiter.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, err := limit.Get(c, key)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
			return
		}
		if ctx.Reached {
			resetAt := time.Unix(ctx.Reset, 0)
			retryAfter := int64(resetAt.Sub(time.Now()).Seconds())

			c.Header("X-RateLimit-Limit", strconv.FormatInt(ctx.Limit, 10))
			c.Header("X-RateLimit-Remaining", strconv.FormatInt(ctx.Remaining, 10))
			c.Header("X-RateLimit-Reset", resetAt.Format(time.RFC1123))
			c.Header("Retry-After", strconv.FormatInt(retryAfter, 10))

			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Too many requests",
				"retry_after": retryAfter,
			})
			return
		}

		c.Next()
	}
}
