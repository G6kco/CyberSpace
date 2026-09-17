package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/G6kco/CyberSpace/internal/middleware"
	"github.com/G6kco/CyberSpace/internal/types"
	"github.com/gin-gonic/gin"
)

func InitRouter(router *gin.Engine) {
	router.Use(middleware.InitCORS())

	router.GET("/health/live", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{
			"status": "alive",
		})
	})

	router.GET("/health/ready", func(ctx *gin.Context) {
		pingContext, cancel := context.WithTimeout(
			ctx.Request.Context(),
			2*time.Second,
		)
		defer cancel()

		if err := types.DBCONN.PingContext(pingContext); err != nil {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not_ready",
				"error":  "database unavailable",
			})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"status": "ready",
		})
	})
}