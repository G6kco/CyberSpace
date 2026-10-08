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
	"github.com/G6kco/CyberSpace/internal/assessments"
	"github.com/G6kco/CyberSpace/internal/auth"
	"github.com/G6kco/CyberSpace/internal/config"
	"github.com/G6kco/CyberSpace/internal/database"
	"github.com/G6kco/CyberSpace/internal/httpapi"
	"github.com/G6kco/CyberSpace/internal/jobs"
	"github.com/G6kco/CyberSpace/internal/labs"
	"github.com/G6kco/CyberSpace/internal/labs/docker"
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
	authRepository := database.NewAuthRepository(db)
	googleLogin, err := auth.NewGoogleLogin(
		context.Background(),
		cfg,
		authRepository,
	)
	if err != nil {
		log.Error("Google login initialization failed", zap.Error(err))
		return fmt.Errorf("initialize Google login: %w", err)
	}
	application.Auth = googleLogin
	log.Info("Google login initialized")

	// Assessments: attendance, dealing, scoring. Labs are requested through
	// the lab tables even when the engine is disabled, so a development
	// server without Docker still runs the whole flow with labs queued.
	labRepository := database.NewLabRepository(db)
	assessmentService, err := assessments.NewService(
		database.NewAssessmentRepository(db),
		labRepository,
		log,
		assessments.Config{AnswerKey: []byte(cfg.AnswerKey), PcapDir: cfg.PcapDir},
	)
	if err != nil {
		return fmt.Errorf("initialize assessments: %w", err)
	}
	if _, err := os.Stat(cfg.PcapDir); err != nil {
		log.Warn("Capture directory unavailable; Wireshark downloads will fail",
			zap.String("dir", cfg.PcapDir), zap.Error(err))
	}
	application.Assessments = assessmentService

	// Gin's debug mode prints every route and warns on startup; production
	// runs quiet.
	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	engine := httpapi.NewRouter(application)
	
	// Background maintenance. The deferred stop runs before the database's
    // deferred Close (defers run last-in, first-out), so the job never uses
    // a closed connection pool.
	jobsContext, stopJobs := context.WithCancel(context.Background())
	jobsDone := make(chan struct{})
	go func() {
		defer close(jobsDone)
		jobs.NewLoginFlowCleanup(authRepository, log, time.Hour, time.Hour).Run(jobsContext)
	}()
	expiryDone := make(chan struct{})
	go func() {
		defer close(expiryDone)
		// Scores attempts whose hour is up; their labs are then stopped.
		assessmentService.RunExpiry(jobsContext, 5*time.Second)
	}()
	
	defer func() {
		stopJobs()
		<-jobsDone
		<-expiryDone
		log.Info("background jobs stopped")
	}()

	// The lab engine starts and removes students' lab containers, so the API
	// must run on the Docker host. Production refuses to start without
	// Docker; development continues without labs, since most work there
	// needs no containers.
	dockerContext, cancelDocker := context.WithTimeout(context.Background(), 10*time.Second)
	labRuntime, err := docker.New(dockerContext)
	cancelDocker()
	switch {
	case err != nil && cfg.AppEnv == "production":
		log.Error("Docker connection failed", zap.Error(err))
		return fmt.Errorf("connect to docker: %w", err)
	case err != nil:
		log.Warn("Docker unreachable; lab engine disabled", zap.Error(err))
	default:
		labEngine := labs.NewEngine(labRepository, labRuntime, log, labs.EngineConfig{})
		labsDone := make(chan struct{})
		go func() {
			defer close(labsDone)
			labEngine.Run(jobsContext)
		}()
		// Registered after the jobs' defer, so it runs first: the engine
		// finishes its in-flight job before the shared context is waited on.
		defer func() {
			stopJobs()
			<-labsDone
			_ = labRuntime.Close()
			log.Info("lab engine stopped")
		}()
		log.Info("Lab engine started")
	}

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
