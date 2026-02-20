package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"go.uber.org/zap"

	httpAdapter "github.com/mehmetymw/event-driven-ns/internal/adapter/http"
	"github.com/mehmetymw/event-driven-ns/internal/adapter/postgres"
	"github.com/mehmetymw/event-driven-ns/internal/adapter/queue"
	"github.com/mehmetymw/event-driven-ns/internal/adapter/ws"
	"github.com/mehmetymw/event-driven-ns/internal/app"
	"github.com/mehmetymw/event-driven-ns/pkg/config"
	"github.com/mehmetymw/event-driven-ns/pkg/logger"
	"github.com/mehmetymw/event-driven-ns/pkg/tracing"
)

func main() {
	cfg := config.Load()

	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tp, err := tracing.InitTracer(ctx, "event-driven-ns", cfg.JaegerEndpoint)
	if err != nil {
		log.Warn("failed to initialize tracer, continuing without tracing", zap.Error(err))
	} else {
		defer func() { _ = tp.Shutdown(ctx) }()
	}

	db, err := postgres.NewConnection(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer func() { _ = db.Close() }()

	runMigrations(cfg.DatabaseURL, log)

	notificationRepo := postgres.NewNotificationRepo(db)
	templateRepo := postgres.NewTemplateRepo(db)
	idempotencyStore := postgres.NewIdempotencyRepo(db)
	producer := queue.NewProducer(cfg.KafkaBrokers)
	defer func() { _ = producer.Close() }()
	wsHub := ws.NewHub()

	notificationService := app.NewNotificationService(
		notificationRepo,
		producer,
		templateRepo,
		idempotencyStore,
		log,
	)

	templateService := app.NewTemplateService(templateRepo, log)
	metricsCollector := app.NewMetricsCollector(notificationRepo)

	notificationHandler := httpAdapter.NewNotificationHandler(notificationService)
	templateHandler := httpAdapter.NewTemplateHandler(templateService)
	healthHandler := httpAdapter.NewHealthHandler(db, cfg.KafkaBrokers)
	metricsHandler := httpAdapter.NewMetricsHandler(metricsCollector)
	wsHandler := httpAdapter.NewWebSocketHandler(wsHub)

	router := httpAdapter.NewRouter(httpAdapter.RouterDeps{
		NotificationHandler: notificationHandler,
		TemplateHandler:     templateHandler,
		HealthHandler:       healthHandler,
		MetricsHandler:      metricsHandler,
		WebSocketHandler:    wsHandler,
		Logger:              log,
	})

	srv := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("starting http server", zap.String("port", cfg.AppPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("http server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", zap.Error(err))
	}

	log.Info("server stopped")
}

func runMigrations(databaseURL string, log *zap.Logger) {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		log.Warn("failed to create migrator", zap.Error(err))
		return
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Warn("migration failed", zap.Error(err))
		return
	}

	log.Info("database migrations applied")
}
