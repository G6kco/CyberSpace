package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/G6kco/CyberSpace/internal/app"
	"github.com/G6kco/CyberSpace/internal/auth"
	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/G6kco/CyberSpace/internal/database"
	"github.com/G6kco/CyberSpace/internal/httpapi"
	"github.com/G6kco/CyberSpace/logger"
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
	log, err := logger.New(cfg.AppEnv)
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}

	defer func() {
		_ = log.Sync()
	}()

	log.Info("Configuration loaded successfully", zap.String("environment", cfg.AppEnv))

	// 3. Create the database connection.
	db, err := database.NewMySQL(cfg.DatabaseURL)
	if err != nil {
		log.Error("Database connection failed", zap.Error(err))
		return fmt.Errorf("connect to database: %w", err)
	}
	log.Info("Database connection pool initialized")

	defer func() {
		if err := db.Close(); err != nil {
			log.Error(
				"Failed to close database connection",
				zap.Error(err),
			)
			return
		}
		log.Info("Database connection closed successfully")
	}()

	// 4. Assemble the application and inject it into the HTTP layer.
	application, err := app.New(cfg, db, log)
	if err != nil {
		log.Error("Application dependency initialization failed", zap.Error(err))
		return fmt.Errorf("initialize application: %w", err)
	}
	log.Info("Application dependencies initialized")

	// 5. Build Google sign-in. NewGoogleLogin performs OIDC discovery, so a
	// server that cannot reach Google's configuration fails here at startup
	// rather than on a user's first login attempt.
	googleLogin, err := auth.NewGoogleLogin(
		context.Background(),
		cfg,
		database.NewAuthRepository(db),
	)
	if err != nil {
		log.Error("Google login initialization failed", zap.Error(err))
		return fmt.Errorf("initialize Google login: %w", err)
	}
	application.Auth = googleLogin
	log.Info("Google login initialized")

	engine := httpapi.NewRouter(application)

	// 6. Construct an explicit HTTP server.
	httpServer := newHTTPServer(cfg.ServerHost, cfg.ServerPort, engine)

	// 7. Start the HTTP server without blocking signal handling.
	serverErrors := make(chan error, 1)

	go func() {
		log.Info(
			"Starting HTTP server",
			zap.String("environment", cfg.AppEnv),
			zap.String("host", cfg.ServerHost),
			zap.Uint("port", cfg.ServerPort),
		)

		err := httpServer.ListenAndServe()

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	// 8. Listen for Ctrl+C, Docker stop, or system termination.
	shutdownContext, stopSignals := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stopSignals()

	select {
	case <-shutdownContext.Done():
		log.Info("Shutdown signal received")

	case err := <-serverErrors:
		log.Error("HTTP server failed", zap.Error(err))
		return fmt.Errorf("HTTP server failed: %w", err)
	}

	// Stop receiving further shutdown signals.
	stopSignals()

	// 9. Allow active requests up to 10 seconds to finish.
	timeoutContext, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	log.Info("Shutting down HTTP server")

	if err := httpServer.Shutdown(timeoutContext); err != nil {
		log.Error(
			"Graceful shutdown failed; forcing server to close",
			zap.Error(err),
		)

		if closeErr := httpServer.Close(); closeErr != nil {
			log.Error("Forced HTTP server close failed", zap.Error(closeErr))
			return fmt.Errorf(
				"force close HTTP server: %w",
				closeErr,
			)
		}

		return fmt.Errorf("graceful shutdown HTTP server: %w", err)
	}

	log.Info("HTTP server stopped successfully")

	return nil
}

func newHTTPServer(host string, port uint, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              net.JoinHostPort(host, strconv.FormatUint(uint64(port), 10)),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
