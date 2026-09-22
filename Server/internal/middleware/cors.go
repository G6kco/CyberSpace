package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func InitCORS(allowedOrigins []string) gin.HandlerFunc {
	if len(allowedOrigins) == 0 {
		return func(ctx *gin.Context) {
			ctx.Next()
		}
	}

	corsConfig := cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "QUERY"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           2 * time.Hour,
	}

	return cors.New(corsConfig)
}
