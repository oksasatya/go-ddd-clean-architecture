package modules

import (
	"expvar"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/oksasatya/go-ddd-clean-architecture/internal/container"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/interface/middleware"
)

type DebugModule struct{}

func NewDebugModule() *DebugModule { return &DebugModule{} }

func (m *DebugModule) Register(rg *gin.RouterGroup) {
	// Public metrics endpoint (expvar), rate-limited per IP
	rl := middleware.RateLimit(container.GetRedis(), 120, time.Minute, middleware.KeyByIP(), nil)
	rg.GET("/debug/vars", rl, gin.WrapH(expvar.Handler()))
}
