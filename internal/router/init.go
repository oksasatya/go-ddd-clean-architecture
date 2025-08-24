package router

import (
	"expvar"
	"time"

	"github.com/gin-gonic/gin"

	appuser "github.com/oksasatya/go-ddd-clean-architecture/internal/application"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/container"
	repouser "github.com/oksasatya/go-ddd-clean-architecture/internal/domain/repository"
	pginfra "github.com/oksasatya/go-ddd-clean-architecture/internal/infrastructure/postgres"
	handlers "github.com/oksasatya/go-ddd-clean-architecture/internal/interface/http"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/interface/middleware"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/router/modules"
)

type UserModuleDeps struct {
	Repo    repouser.UserRepository
	Service *appuser.Service
	Handler *handlers.UserHandler
}

func buildUserDeps() UserModuleDeps {
	repo := pginfra.NewUserRepository(container.GetPGPool())

	service := appuser.NewService(
		repo,
		container.GetJWT(),
		container.GetGCS(),
		container.GetConfig().GCSBucket,
		container.GetRedis(),
		container.GetLogger(),
		container.GetES(),
		container.GetConfig().ESUsersIndex,
	)

	handler := handlers.NewUserHandler(
		service,
		container.GetJWT(),
		container.GetLogger(),
		container.GetConfig().CookieDomain,
		container.GetConfig().CookieSecure,
		container.GetRabbitPub(),
		container.GetConfig(),
		container.GetRedis(),
	)

	return UserModuleDeps{
		Repo:    repo,
		Service: service,
		Handler: handler,
	}
}

func buildAuthHandler(repo repouser.UserRepository) *handlers.AuthHandler {
	return handlers.NewAuthHandler(
		repo,
		container.GetRedis(),
		container.GetLogger(),
		container.GetConfig(),
		container.GetRabbitPub(),
		container.GetPGPool(),
	)
}

// InitModules initializes all application modules and registers them with the router registry
// This function should be called once during application startup to wire up all modules
func InitModules(r *Registry) {
	userDeps := buildUserDeps()
	r.Add(modules.New(userDeps.Handler, container.GetJWT()))
	// Email module
	if container.GetRabbitPub() != nil {
		emailHandler := handlers.NewEmailHandler(container.GetRabbitPub(), container.GetLogger(), container.GetConfig())
		r.Add(modules.NewEmailModule(emailHandler, container.GetJWT()))
	}
	// Auth module
	authHandler := buildAuthHandler(userDeps.Repo)
	r.Add(modules.NewAuthModule(authHandler, container.GetJWT()))
	// Debug module (under /api)
	r.Add(modules.NewDebugModule())
	// Root-level alias for expvar metrics
	rl := middleware.RateLimit(container.GetRedis(), 120, time.Minute, middleware.KeyByIP(), nil)
	r.Engine.GET("/debug/vars", rl, gin.WrapH(expvar.Handler()))
}
