package router

import (
	"net/http"

	"github.com/G6kco/CyberSpace/internal/middleware"
	"github.com/gin-gonic/gin"
)

func LoadRouter(router *gin.Engine) {
	router.Use(middleware.InitCORS())
	
	router.GET("/health/live", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"Status": "accepted"})
	})	
}