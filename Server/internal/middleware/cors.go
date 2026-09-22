package middleware

import (
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func InitCORS(rawOrigins string) gin.HandlerFunc {
	allowedOrigins := []string{}
	if rawOrigins = strings.TrimSpace(rawOrigins); rawOrigins != "" {
		parts := strings.Split(rawOrigins, ",")
		parsedOrigins := make([]string, 0, len(parts))
		for _, origin := range parts {
			trimmedOrigin := strings.TrimSpace(origin)
			if trimmedOrigin != "" {
				parsedOrigins = append(parsedOrigins, trimmedOrigin)
			}
		}
		if len(parsedOrigins) > 0 {
			allowedOrigins = parsedOrigins
		}
	}
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
