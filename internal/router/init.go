package router

import (
	appuser "boilerplate-go-pgsql/internal/application"
	"boilerplate-go-pgsql/internal/container"
	repouser "boilerplate-go-pgsql/internal/domain/repository"
	pginfra "boilerplate-go-pgsql/internal/infrastructure/postgres"
	handlers "boilerplate-go-pgsql/internal/interface/http"
	usermodule "boilerplate-go-pgsql/internal/router/modules/user"
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
