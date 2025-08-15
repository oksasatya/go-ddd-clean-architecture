package router

import (
	appuser "github.com/oksasatya/go-ddd-clean-architecture/internal/application"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/container"
	repouser "github.com/oksasatya/go-ddd-clean-architecture/internal/domain/repository"
	pginfra "github.com/oksasatya/go-ddd-clean-architecture/internal/infrastructure/postgres"
	handlers "github.com/oksasatya/go-ddd-clean-architecture/internal/interface/http"
	usermodule "github.com/oksasatya/go-ddd-clean-architecture/internal/router/modules/user"
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
	)

	handler := handlers.NewUserHandler(
		service,
		container.GetJWT(),
		container.GetLogger(),
		container.GetConfig().CookieDomain,
		container.GetConfig().CookieSecure,
	)

	return UserModuleDeps{
		Repo:    repo,
		Service: service,
		Handler: handler,
	}
}

// InitModules initializes all application modules and registers them with the router registry
// This function should be called once during application startup to wire up all modules
func InitModules(r *Registry) {
	userDeps := buildUserDeps()
	r.Add(usermodule.New(userDeps.Handler, container.GetJWT()))
}
