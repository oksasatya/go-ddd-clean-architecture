package modules

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/oksasatya/go-ddd-clean-architecture/internal/container"
	handlers "github.com/oksasatya/go-ddd-clean-architecture/internal/interface/http"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/interface/middleware"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
)

type EmailModule struct {
	Handler *handlers.EmailHandler
	JWT     *helpers.JWTManager
}

func NewEmailModule(h *handlers.EmailHandler, jwt *helpers.JWTManager) *EmailModule {
	return &EmailModule{Handler: h, JWT: jwt}
}

func (m *EmailModule) Register(rg *gin.RouterGroup) {
	// Protected email endpoints
	auth := rg.Group("/")
	auth.Use(middleware.Auth(container.GetRedis(), m.JWT))
	auth.Use(
		middleware.RateLimit(container.GetRedis(), 60, time.Minute, middleware.KeyByUserID(), nil),
	)
	{
		auth.POST("/email/send", m.Handler.Send)
	}
}
