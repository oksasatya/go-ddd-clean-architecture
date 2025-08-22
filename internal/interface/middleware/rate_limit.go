package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/oksasatya/go-ddd-clean-architecture/pkg/response"
)

// ipFromCtx extracts the client IP from Gin context, falling back to "unknown"
func ipFromCtx(c *gin.Context) string {
	if ip := c.GetString("real_ip"); ip != "" {
		return ip
	}
	if ip := c.ClientIP(); ip != "" {
		return ip
	}
	return "unknown"
}

func normalizePath(c *gin.Context) string {
	if fp := c.FullPath(); fp != "" {
		return fp
	}
	return c.Request.URL.Path
}

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
		return "rl:path:" + normalizePath(c) + ":ip:" + ip
	}
}

func KeyByUserID() KeyFunc {
	return func(c *gin.Context) string {
		uid := c.GetString("userID")
		if uid == "" {
			return "rl:user:anon:ip:" + ipFromCtx(c)
		}
		return "rl:user:" + uid
	}
}

// Lua script: atomic INCR + set EXPIRE jika baru
var incrExpireScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return current
`)

type AllowFunc func(*gin.Context) bool // return true for bypass limit

// RateLimit with:
// - atomic redis (lua)
// - standard headers (limit/remaining/reset)
// - optional allowlist bypass & method skip
func RateLimit(rdb *redis.Client, max int, window time.Duration, keyFn KeyFunc, allow AllowFunc) gin.HandlerFunc {
	if rdb == nil || max <= 0 || window <= 0 || keyFn == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		// optional bypass: health, internal IP, admin, dsb
		if allow != nil && allow(c) {
			c.Next()
			return
		}

		// skip OPTIONS
		if strings.EqualFold(c.Request.Method, http.MethodOptions) {
			c.Next()
			return
		}

		ctx := c.Request.Context()
		key := keyFn(c)

		// atomic increment + set ttl (ms)
		countI, err := incrExpireScript.Run(ctx, rdb, []string{key}, window.Milliseconds()).Result()
		if err != nil {
			// fail-open kalau redis error
			c.Next()
			return
		}
		count := toInt(countI)

		// TTL untuk header reset
		ttl, _ := rdb.TTL(ctx, key).Result()
		resetSec := 0
		if ttl > 0 {
			resetSec = int(ttl.Seconds())
		}

		// Standard headers
		// https://datatracker.ietf.org/doc/html/rfc6585#section-4
		// https://tools.ietf.org/html/draft-ietf-httpapi-r
		c.Header("X-RateLimit-Limit", strconv.Itoa(max))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(max-int(count)))
		c.Header("X-RateLimit-Reset", strconv.Itoa(resetSec))

		// Exceeded
		if int(count) > max {
			if resetSec > 0 {
				c.Header("Retry-After", strconv.Itoa(resetSec))
			}
			response.Error[any](c, http.StatusTooManyRequests, "rate limit exceeded", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

func toInt(v interface{}) int {
	switch x := v.(type) {
	case int64:
		return int(x)
	case int:
		return x
	case string:
		i, _ := strconv.Atoi(x)
		return i
	}
	return 0
}
