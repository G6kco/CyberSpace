package httpapi

import (
	"github.com/G6kco/CyberSpace/internal/app"
	"github.com/G6kco/CyberSpace/internal/middleware"
	"github.com/gin-gonic/gin"
)

// NewRouter builds the HTTP transport with dependencies supplied explicitly.
func NewRouter(application *app.App) *gin.Engine {
	router := gin.New()
	// In production nginx on the same host proxies every request, so the
	// forwarding headers are trusted only from loopback. nginx overwrites
	// them, so a client cannot spoof the IP recorded in audit logs. A
	// request from anywhere else keeps its connection address.
	_ = router.SetTrustedProxies([]string{"127.0.0.1", "::1"})
	router.Use(gin.Recovery())
	router.Use(middleware.InitCORS(application.Config.AllowedOrigins))

	health := newHealthHandler(application.DB, application.Logger)
	router.GET("/health/live", health.live)
	router.GET("/health/ready", health.ready)

	// Authentication is optional here so that router tests can exercise the
	// health endpoints without reaching Google. main.go always injects it.
	if application.Auth != nil {
		handler := &authHandler{
			login:       application.Auth,
			logger:      application.Logger,
			frontendURL: application.Config.FrontendURL,
			secure:      application.Config.AppEnv == "production",
		}

		v1 := router.Group("/api/v1")
		v1.GET("/auth/google", handler.start)
		v1.GET("/auth/google/callback", handler.callback)

		protected := v1.Group("")
		protected.Use(handler.requireSession())
		protected.GET("/me", handler.me)
		protected.DELETE("/auth/session", handler.logout)

		if application.Assessments != nil {
			guarded := protected.Group("", requireOrigin(application.Config.FrontendURL))
			registerAssessmentRoutes(guarded, &assessmentHandler{
				service: application.Assessments,
				logger:  application.Logger,
			})
			application.Logger.Info("Assessment routes registered")
		}
		if application.Levels != nil {
			guarded := protected.Group("", requireOrigin(application.Config.FrontendURL))
			registerLevelRoutes(guarded, &levelHandler{
				service: application.Levels,
				logger:  application.Logger,
			})
			application.Logger.Info("Level builder routes registered")
		}

		application.Logger.Info("Authentication routes registered")
	}

	application.Logger.Info("Router initialized")
	return router
}
