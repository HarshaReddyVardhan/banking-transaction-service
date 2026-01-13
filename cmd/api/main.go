package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"

	"github.com/banking/transaction-service/internal/api"
	"github.com/banking/transaction-service/internal/config"
	"github.com/banking/transaction-service/internal/events"
	"github.com/banking/transaction-service/internal/repository"
	"github.com/banking/transaction-service/internal/service"
)

func main() {
	// 1. Initialize Logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// 2. Load Config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	// 3. Setup Database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	repo, err := repository.NewPostgresRepository(ctx, cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer repo.Close()

	// 4. Setup Kafka Producer
	producerCfg := events.ProducerConfig{
		Brokers: cfg.Kafka.Brokers,
		TopicConfig: events.TopicConfig{
			TransferInitiatedTopic: cfg.Kafka.TransferInitiatedTopic,
			TransferApprovedTopic:  cfg.Kafka.TransferApprovedTopic,
			TransferRejectedTopic:  cfg.Kafka.TransferRejectedTopic,
			TransferCompletedTopic: cfg.Kafka.TransferCompletedTopic,
			TransferWaitingTopic:   cfg.Kafka.TransferWaitingTopic,
			AuditTopic:             cfg.Kafka.AuditTopic,
		},
		BufferSize:       100,
		RequireAcks:      -1, // WaitForAll
		EnableIdempotent: true,
		MaxRetries:       3,
		RetryBackoff:     100 * time.Millisecond,
	}

	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:    "kafka-producer",
		Timeout: cfg.Resilience.CircuitBreakerTimeout,
	})

	producer, err := events.NewProducer(producerCfg, cb, logger)
	if err != nil {
		logger.Fatal("Failed to modify producer", zap.Error(err))
	}
	defer producer.Close()

	// 5. Setup Service
	svc := service.NewTransactionService(repo, producer, logger, cfg.Transaction)

	// 6. Setup API Handler
	handler := api.NewHandler(svc)

	// 7. Setup HTTP Server
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())

	api.RegisterRoutes(e, handler)

	// 8. Start Server (Graceful Shutdown)
	go func() {
		if err := e.Start(cfg.Server.Host + ":" + "8081"); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("Shutting down the server", zap.Error(err))
		}
	}()
	logger.Info("Server started on port 8081")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel = context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exiting")
}
