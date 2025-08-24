package modules

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/oksasatya/go-ddd-clean-architecture/internal/container"
	handlers "github.com/oksasatya/go-ddd-clean-architecture/internal/interface/http"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/interface/middleware"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
)

type AuthModule struct {
	Handler *handlers.AuthHandler
	JWT     *helpers.JWTManager
}

func NewAuthModule(h *handlers.AuthHandler, jwt *helpers.JWTManager) *AuthModule {
	return &AuthModule{Handler: h, JWT: jwt}
}

func (m *AuthModule) Register(rg *gin.RouterGroup) {
	// Public endpoints with IP-based rate limits
	verifyConfirmLimiter := middleware.RateLimit(container.GetRedis(), 30, time.Minute, middleware.KeyByIPAndPath(), nil)
	resetInitLimiter := middleware.RateLimit(container.GetRedis(), 5, time.Minute, middleware.KeyByIPAndPath(), nil)
	resetConfirmLimiter := middleware.RateLimit(container.GetRedis(), 30, time.Minute, middleware.KeyByIPAndPath(), nil)

	rg.POST("/auth/verify/confirm", verifyConfirmLimiter, m.Handler.VerifyConfirm)
	rg.POST("/auth/reset/init", resetInitLimiter, m.Handler.ResetInit)
	rg.POST("/auth/reset/confirm", resetConfirmLimiter, m.Handler.ResetConfirm)

	// Protected verify init with user-based rate limit
	auth := rg.Group("/")
	auth.Use(middleware.Auth(container.GetRedis(), m.JWT))
	auth.Use(middleware.RateLimit(container.GetRedis(), 5, time.Minute, middleware.KeyByUserID(), nil))
	{
		auth.POST("/auth/verify/init", m.Handler.VerifyInit)
	}
}
