package httpapi

import (
	"github.com/G6kco/CyberSpace/internal/app"
	"github.com/G6kco/CyberSpace/internal/middleware"
	"github.com/gin-gonic/gin"
)

// NewRouter builds the HTTP transport with dependencies supplied explicitly.
func NewRouter(application *app.App) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.InitCORS(application.Config.AllowedOrigins))

	health := newHealthHandler(application.DB, application.Logger)
	router.GET("/health/live", health.live)
	router.GET("/health/ready", health.ready)

	application.Logger.Info("Router initialized")
	return router
}
