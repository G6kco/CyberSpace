package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/G6kco/CyberSpace/internal/app"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type healthHandler struct {
	db     app.Database
	logger *zap.Logger
}

func newHealthHandler(db app.Database, logger *zap.Logger) *healthHandler {
	return &healthHandler{db: db, logger: logger}
}

func (handler *healthHandler) live(ctx *gin.Context) {
	handler.logger.Debug("Liveness check succeeded")
	ctx.JSON(http.StatusOK, gin.H{"status": "alive"})
}
func (handler *healthHandler) ready(ctx *gin.Context) {
	pingContext, cancel := context.WithTimeout(ctx.Request.Context(), 2*time.Second)
	defer cancel()

	if err := handler.db.PingContext(pingContext); err != nil {
		handler.logger.Error("Readiness check failed", zap.Error(err))
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not_ready",
			"error":  "database unavailable",
		})
		return
	}

	handler.logger.Debug("Readiness check succeeded")
	ctx.JSON(http.StatusOK, gin.H{"status": "ready"})
}
