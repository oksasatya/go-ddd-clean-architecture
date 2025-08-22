package middleware

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// RealIP sets the real client IP into Gin context (key: "real_ip").
// Priority:
// 1) CF-Connecting-IP (Cloudflare)
// 2) X-Forwarded-For (left-most)
// 3) fallback to c.ClientIP()
func RealIP() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1) Cloudflare header
		if cf := strings.TrimSpace(c.GetHeader("CF-Connecting-IP")); cf != "" {
			if ip := net.ParseIP(cf); ip != nil {
				c.Set("real_ip", ip.String())
				c.Next()
				return
			}
		}
		// 2) X-Forwarded-For: take left-most
		if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				first := strings.TrimSpace(parts[0])
				if ip := net.ParseIP(first); ip != nil {
					c.Set("real_ip", ip.String())
					c.Next()
					return
				}
			}
		}
		// 3) Fallback
		c.Set("real_ip", c.ClientIP())
		c.Next()
	}
}
