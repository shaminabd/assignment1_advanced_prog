package app

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"

	"notification-service/internal/infrastructure/rabbitmq"
)

func Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@localhost:5672/"
	}

	cons, err := rabbitmq.NewConsumer(amqpURL)
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
