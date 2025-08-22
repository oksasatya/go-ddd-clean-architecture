package middleware

import (
	"github.com/gin-gonic/gin"
	"net"
)

// AllowPrivateIP returns a middleware function that allows requests
// from private IP addresses. It checks if the client's IP is a private
func AllowPrivateIP() AllowFunc {
	return func(c *gin.Context) bool {
		ip := ipFromCtx(c)
		parsed := net.ParseIP(ip)
		if parsed == nil {
			return false
		}
		// 10.0.0.0/8, 172.16/12, 192.168/16, loopback
		private := parsed.IsLoopback() ||
			parsed.IsPrivate()
		return private
	}
}
