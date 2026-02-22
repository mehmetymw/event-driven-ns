package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/mehmetymw/event-driven-ns/internal/adapter/postgres"
	"github.com/mehmetymw/event-driven-ns/internal/adapter/provider"
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

	tp, err := tracing.InitTracer(ctx, "event-driven-ns-worker", cfg.JaegerEndpoint)
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

	notificationRepo := postgres.NewNotificationRepo(db)
	webhookProvider := provider.NewWebhookProvider(cfg.WebhookURL)
	wsHub := ws.NewHub()
	metricsCollector := app.NewMetricsCollector(notificationRepo)

	deliveryService := app.NewDeliveryService(
		notificationRepo,
		webhookProvider,
		wsHub,
		metricsCollector,
		log,
	)

	schedulerProducer := queue.NewProducer(cfg.KafkaBrokers)
	defer func() { _ = schedulerProducer.Close() }()

	scheduler := app.NewScheduler(notificationRepo, schedulerProducer, log)
	go scheduler.Run(ctx)

	consumer := queue.NewConsumer(queue.ConsumerConfig{
		Brokers:        cfg.KafkaBrokers,
		Group:          cfg.KafkaConsumerGroup,
		RatePerChannel: cfg.RateLimitPerChannel,
		Logger:         log,
	})

	go func() {
		log.Info("starting kafka consumer",
			zap.Strings("brokers", cfg.KafkaBrokers),
			zap.String("group", cfg.KafkaConsumerGroup),
		)
		if err := consumer.Start(ctx, deliveryService.ProcessDelivery); err != nil {
			if ctx.Err() == nil {
				log.Error("consumer stopped unexpectedly", zap.Error(err))
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down worker gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	cancel()

	if err := consumer.Stop(shutdownCtx); err != nil {
		log.Error("consumer shutdown error", zap.Error(err))
	}

	log.Info("worker stopped")
}
