package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"boilerplate-go-pgsql/pkg/helpers"
)

// KeyFunc builds a rate-limit key from the request
// Example: combine client IP and route path for more granular limiting
type KeyFunc func(c *gin.Context) string

// KeyByIP returns a key function that limits by client IP only
func KeyByIP() KeyFunc {
	return func(c *gin.Context) string {
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}
		return "rl:ip:" + ip
	}
}

// KeyByIPAndPath returns a key function that limits by client IP and request path
func KeyByIPAndPath() KeyFunc {
	return func(c *gin.Context) string {
		ip := c.ClientIP()
		if ip == "" {
			ip = "unknown"
		}
		return "rl:path:" + c.FullPath() + ":ip:" + ip
	}
}

// RateLimit creates a Gin middleware that limits requests using Redis as a shared counter.
// - rdb: Redis client
// - max: maximum number of requests allowed within the window
// - window: time window for the limit
// - keyFn: function to construct a redis key for the caller
func RateLimit(rdb *redis.Client, max int, window time.Duration, keyFn KeyFunc) gin.HandlerFunc {
	if rdb == nil || max <= 0 || window <= 0 || keyFn == nil {
		// No-op middleware if misconfigured
		return func(c *gin.Context) { c.Next() }
	}

	return func(c *gin.Context) {
		ctx := c.Request.Context()
		key := keyFn(c)

		// INCR the counter and set expiry on first hit
		count, err := rdb.Incr(ctx, key).Result()
		if err != nil {
			// On Redis error, fail open
			c.Next()
			return
		}
		if count == 1 {
			_ = rdb.Expire(ctx, key, window).Err()
		}

		if int(count) > max {
			// Optionally set Retry-After header to remaining window
			ttl, _ := rdb.TTL(ctx, key).Result()
			if ttl > 0 {
				c.Header("Retry-After", strconv.Itoa(int(ttl.Seconds())))
			}
			resp := helpers.Error[any](c, http.StatusTooManyRequests, "rate limit exceeded", nil)
			c.AbortWithStatusJSON(resp.Status, resp)
			return
		}

		// Continue
		c.Next()
	}
}
