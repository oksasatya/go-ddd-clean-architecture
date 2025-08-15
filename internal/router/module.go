package router

import "github.com/gin-gonic/gin"

// Module describes a feature module that can register its routes on a RouterGroup
type Module interface {
	Register(rg *gin.RouterGroup)
}
