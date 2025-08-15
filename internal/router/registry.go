package router

import "github.com/gin-gonic/gin"

type Registry struct {
	Engine      *gin.Engine
	API         *gin.RouterGroup
	middlewares []gin.HandlerFunc
	modules     []Module
}

func NewRegistry(engine *gin.Engine) *Registry {
	api := engine.Group("/api")
	return &Registry{Engine: engine, API: api}
}

func (r *Registry) Use(mw ...gin.HandlerFunc) {
	r.middlewares = append(r.middlewares, mw...)
}

func (r *Registry) Add(mod Module) {
	r.modules = append(r.modules, mod)
}

func (r *Registry) RegisterAll() {
	if len(r.middlewares) > 0 {
		r.API.Use(r.middlewares...)
	}
	for _, m := range r.modules {
		m.Register(r.API)
	}
}
