package http

import "github.com/gin-gonic/gin"

type Registrar interface {
	HttpRegister(g *gin.RouterGroup)
}
