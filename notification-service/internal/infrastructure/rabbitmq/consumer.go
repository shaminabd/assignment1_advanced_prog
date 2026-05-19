package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/google/uuid"

	"notification-service/internal/event"
	"notification-service/internal/infrastructure/redis"
	"notification-service/internal/provider"
)

const (
	exchangeName = "payment.events"
	queueName    = "payment.completed"
	routingKey   = "payment.completed"
)

type Consumer struct {
	conn        *amqp.Connection
	ch          *amqp.Channel
	sender      provider.EmailSender
	idempotency *redis.IdempotencyStore
	retryMax    int
	retryBase   time.Duration
	workers     int
}

func closeAndWrap(conn *amqp.Connection, ch *amqp.Channel, format string, err error) error {
	if ch != nil {
		_ = ch.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}
	return fmt.Errorf(format, err)
}

func NewConsumer(amqpURL string, sender provider.EmailSender, idempotency *redis.IdempotencyStore) (*Consumer, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, closeAndWrap(conn, nil, "rabbitmq channel: %w", err)
	}

	workers := workersFromEnv()
	if err := ch.Qos(workers, 0, false); err != nil {
		return nil, closeAndWrap(conn, ch, "qos: %w", err)
	}

	if err := ch.ExchangeDeclare(
		exchangeName,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, closeAndWrap(conn, ch, "exchange declare: %w", err)
	}

	if _, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, closeAndWrap(conn, ch, "queue declare: %w", err)
	}

	if err := ch.QueueBind(queueName, routingKey, exchangeName, false, nil); err != nil {
		return nil, closeAndWrap(conn, ch, "queue bind: %w", err)
	}

	return &Consumer{
		conn:        conn,
		ch:          ch,
		sender:      sender,
		idempotency: idempotency,
		retryMax:    retryMaxFromEnv(),
		retryBase:   retryBaseFromEnv(),
		workers:     workers,
	}, nil
}

func workersFromEnv() int {
	n := 5
	if raw := os.Getenv("NOTIFICATION_WORKERS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			n = parsed
		}
	}
	return n
}

func retryMaxFromEnv() int {
	max := 5
	if raw := os.Getenv("NOTIFICATION_RETRY_MAX"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			max = parsed
		}
	}
	return max
}

func retryBaseFromEnv() time.Duration {
	base := 2 * time.Second
	if raw := os.Getenv("NOTIFICATION_RETRY_BASE_DELAY"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			base = parsed
		}
	}
	return base
}

func (c *Consumer) Run(ctx context.Context) error {
	deliveries, err := c.ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	log.Printf("notification-service parallel workers=%d retry_max=%d", c.workers, c.retryMax)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("deliveries channel closed")
			}
			go c.handleDelivery(ctx, d)
		}
	}
}

func (c *Consumer) handleDelivery(ctx context.Context, d amqp.Delivery) {
	var ev event.PaymentCompleted
	if err := json.Unmarshal(d.Body, &ev); err != nil {
		log.Printf("notification: invalid payload: %v", err)
		_ = d.Nack(false, false)
		return
	}

	eventID := d.MessageId
	if eventID == "" {
		eventID = uuid.New().String()
	}
	log.Printf("[Consumer] [goroutine] Processing event %s for order %s", eventID, ev.OrderID)

	claimed, alreadySent, err := c.idempotency.TryClaim(ctx, ev.OrderID)
	if err != nil {
		log.Printf("notification: idempotency claim: %v", err)
		_ = d.Nack(false, true)
		return
	}
	if alreadySent {
		_ = d.Ack(false)
		return
	}
	if !claimed {
		_ = d.Nack(false, true)
		return
	}

	amountStr := fmt.Sprintf("$%.2f", float64(ev.Amount)/100)
	subject := ev.OrderID
	body := amountStr

	var sendErr error
	for attempt := 0; attempt < c.retryMax; attempt++ {
		if attempt > 0 {
			delay := c.retryBase * time.Duration(1<<(attempt-1))
			log.Printf("notification: retry %d/%d for order %s in %s", attempt, c.retryMax-1, ev.OrderID, delay)
			select {
			case <-ctx.Done():
				_ = c.idempotency.Release(ctx, ev.OrderID)
				_ = d.Nack(false, true)
				return
			case <-time.After(delay):
			}
		}

		sendErr = c.sender.Send(ctx, ev.CustomerEmail, subject, body)
		if sendErr == nil {
			break
		}
		log.Printf("notification: send failed for order %s (attempt %d): %v", ev.OrderID, attempt+1, sendErr)
	}

	if sendErr != nil {
		log.Printf("notification: exhausted retries for order %s: %v", ev.OrderID, sendErr)
		_ = c.idempotency.Release(ctx, ev.OrderID)
		_ = d.Nack(false, false)
		return
	}

	if err := c.idempotency.MarkSent(ctx, ev.OrderID); err != nil {
		log.Printf("notification: mark sent: %v", err)
		_ = c.idempotency.Release(ctx, ev.OrderID)
		_ = d.Nack(false, true)
		return
	}

	if err := d.Ack(false); err != nil {
		log.Printf("notification: ack: %v", err)
	}
}

func (c *Consumer) Close() error {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
