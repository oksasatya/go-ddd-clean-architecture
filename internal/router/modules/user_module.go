package modules

import (
	"github.com/gin-gonic/gin"
	"time"

	"github.com/oksasatya/go-ddd-clean-architecture/internal/container"

	handlers "github.com/oksasatya/go-ddd-clean-architecture/internal/interface/http"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/interface/middleware"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
)

// Module wires user HTTP handlers and JWT middleware into routes
// Public: POST /api/login, POST /api/refresh
// Protected: POST /api/logout, GET /api/profile, PUT /api/profile
// All routes are registered under the given RouterGroup (usually /api)

type Module struct {
	Handler *handlers.UserHandler
	JWT     *helpers.JWTManager
}

func New(h *handlers.UserHandler, jwt *helpers.JWTManager) *Module {
	return &Module{Handler: h, JWT: jwt}
}

func (m *Module) Register(rg *gin.RouterGroup) {
	// Public with rate limiting
	loginLimiter := middleware.RateLimit(container.GetRedis(), 5, time.Minute, middleware.KeyByIPAndPath())
	refreshLimiter := middleware.RateLimit(container.GetRedis(), 20, time.Minute, middleware.KeyByIPAndPath())

	rg.POST("/login", loginLimiter, m.Handler.Login)
	rg.POST("/refresh", refreshLimiter, m.Handler.Refresh)

	// Protected
	auth := rg.Group("/")
	auth.Use(middleware.Auth(container.GetRedis(), m.JWT))
	// Apply a softer per-IP limiter to all protected routes
	auth.Use(middleware.RateLimit(container.GetRedis(), 120, time.Minute, middleware.KeyByIP()))
	{
		auth.POST("/logout", m.Handler.Logout)
		auth.GET("/profile", m.Handler.GetProfile)
		auth.PUT("/profile", m.Handler.UpdateProfile)
	}
}
