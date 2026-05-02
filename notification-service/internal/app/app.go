package app

import (
	"context"
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"

	"notification-service/internal/infrastructure/rabbitmq"
	"notification-service/internal/repository"
)

func Run() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/notification_db?sslmode=disable"
	}

	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		amqpURL = "amqp://guest:guest@localhost:5672/"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS processed_events (
	event_id UUID PRIMARY KEY,
	processed_at TIMESTAMP NOT NULL DEFAULT NOW()
)`); err != nil {
		log.Fatal(err)
	}

	repo := repository.NewProcessedRepository(db)

	cons, err := rabbitmq.NewConsumer(amqpURL, repo)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = cons.Close() }()

	log.Println("notification-service consuming queue payment.completed (manual ack, durable queue)")

	ctx := context.Background()
	if err := cons.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
