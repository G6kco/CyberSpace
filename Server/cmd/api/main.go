package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/G6kco/CyberSpace/internal/database"
	"github.com/G6kco/CyberSpace/internal/httpapi"
	"github.com/G6kco/CyberSpace/internal/types"
	"github.com/G6kco/CyberSpace/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "application stopped: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// 1. Load application configuration.
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	// 2. Initialize the logger before starting other dependencies.
	if cfg.AppEnv == "" {
		return errors.New("APP_ENV cannot be empty")
	}

	logger.LoadLogger(cfg.AppEnv)

	if types.LOG == nil {
		return errors.New("logger initialization failed")
	}

	defer func() {
		_ = types.LOG.Sync()
	}()

	// 3. Create the database connection.
	db, err := database.NewMySQL(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}

	defer func() {
		if err := db.Close(); err != nil {
			types.LOG.Error(
				"Failed to close database connection",
				zap.Error(err),
			)
		}
	}()

	// Temporary compatibility with your existing global connection.
	types.DBCONN = db

	// 4. Create the Gin router.
	engine := gin.New()

	// Prevent a panic inside a handler from crashing the whole API.
	engine.Use(gin.Recovery())

	httpapi.InitRouter(engine)

	types.LOG.Info("Router loaded")

	// 5. Construct an explicit HTTP server.
	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.ServerPort),
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// 6. Start the HTTP server without blocking signal handling.
	serverErrors := make(chan error, 1)

	go func() {
		types.LOG.Info(
			"Starting HTTP server",
			zap.String("environment", cfg.AppEnv),
			zap.Uint("port", cfg.ServerPort),
		)

		err := httpServer.ListenAndServe()

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 7. Listen for Ctrl+C, Docker stop, or system termination.
	shutdownContext, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	select {
	case <-shutdownContext.Done():
		types.LOG.Info("Shutdown signal received")

	case err := <-serverErrors:
		return fmt.Errorf("HTTP server failed: %w", err)
	}

	// Stop receiving further shutdown signals.
	stopSignals()

	// 8. Allow active requests up to 10 seconds to finish.
	timeoutContext, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	types.LOG.Info("Shutting down HTTP server")

	if err := httpServer.Shutdown(timeoutContext); err != nil {
		types.LOG.Error(
			"Graceful shutdown failed; forcing server to close",
			zap.Error(err),
		)

		if closeErr := httpServer.Close(); closeErr != nil {
			return fmt.Errorf(
				"force close HTTP server: %w",
				closeErr,
			)
		}

		return fmt.Errorf("graceful shutdown HTTP server: %w", err)
	}

	types.LOG.Info("HTTP server stopped successfully")

	return nil
}