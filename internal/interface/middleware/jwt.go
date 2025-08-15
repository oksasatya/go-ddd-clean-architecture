package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"boilerplate-go-pgsql/pkg/helpers"
)

const CtxUserIDKey = "userID"

// JWTAuth reads access_token cookie, validates it, and injects user ID into context
func JWTAuth(jwt *helpers.JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("access_token")
		if err != nil || token == "" {
			resp := helpers.Error[any](c, http.StatusUnauthorized, "missing access token", nil)
			c.AbortWithStatusJSON(resp.Status, resp)
			return
		}
		claims, err := jwt.ParseAccessToken(token)
		if err != nil {
			resp := helpers.Error[any](c, http.StatusUnauthorized, "invalid access token", err.Error())
			c.AbortWithStatusJSON(resp.Status, resp)
			return
		}
		c.Set(CtxUserIDKey, claims.UserID)
		c.Next()
	}
}
