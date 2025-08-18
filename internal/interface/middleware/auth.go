package middleware

import (
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/response"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
)

// Auth validates access token and ensures an active session exists in Redis.
// It sets userID, userName, and userEmail in the Gin context on success.
func Auth(rdb *redis.Client, jwt *helpers.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			resp := response.Error[any](c, http.StatusUnauthorized, "missing access token", nil)
			c.AbortWithStatusJSON(resp.Status, resp)
			return
		}
		claims, err := jwt.ParseAccessToken(token)
		if err != nil {
			resp := response.Error[any](c, http.StatusUnauthorized, "invalid access token", err.Error())
			c.AbortWithStatusJSON(resp.Status, resp)
			return
		}

		// Retrieve session from Redis as a hash
		key := "user:session:" + claims.UserID
		data, err := rdb.HGetAll(c.Request.Context(), key).Result()
		if err != nil || len(data) == 0 {
			resp := response.Error[any](c, http.StatusUnauthorized, "session not found", nil)
			c.AbortWithStatusJSON(resp.Status, resp)
			return
		}

		c.Set("userID", data["user_id"])  // required by handlers
		c.Set("userName", data["name"])   // extra convenience
		c.Set("userEmail", data["email"]) // extra convenience
		c.Next()
	}
}
