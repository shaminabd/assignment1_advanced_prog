package app

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	goredis "github.com/redis/go-redis/v9"

	redisinfra "notification-service/internal/infrastructure/redis"
	"notification-service/internal/infrastructure/rabbitmq"
	"notification-service/internal/provider"
)

func Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@localhost:5672/"
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	redisClient := goredis.NewClient(&goredis.Options{Addr: redisAddr})
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(pingCtx).Err(); err != nil {
		log.Fatalf("redis ping: %v", err)
	}
	defer func() { _ = redisClient.Close() }()

	sender, err := provider.NewEmailSenderFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	idempotency := redisinfra.NewIdempotencyStore(redisClient)

	cons, err := rabbitmq.NewConsumer(amqpURL, sender, idempotency)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = cons.Close() }()

	log.Println("notification-service consuming queue payment.completed (manual ack, durable queue)")

	if err := cons.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}

	log.Println("notification-service stopped")
}
