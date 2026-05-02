package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"notification-service/internal/event"
	"notification-service/internal/repository"
)

const (
	exchangeName = "payment.events"
	queueName    = "payment.completed"
	routingKey   = "payment.completed"
)

type Consumer struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	repo *repository.ProcessedRepository
}

func NewConsumer(amqpURL string, repo *repository.ProcessedRepository) (*Consumer, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
	}

	if err := ch.Qos(1, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("qos: %w", err)
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
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("exchange declare: %w", err)
	}

	if _, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("queue declare: %w", err)
	}

	if err := ch.QueueBind(queueName, routingKey, exchangeName, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("queue bind: %w", err)
	}

	return &Consumer{conn: conn, ch: ch, repo: repo}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	deliveries, err := c.ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("deliveries channel closed")
			}
			c.handleDelivery(ctx, d)
		}
	}
}

func (c *Consumer) handleDelivery(ctx context.Context, d amqp.Delivery) {
	var ev event.PaymentCompleted
	if err := json.Unmarshal(d.Body, &ev); err != nil {
		log.Printf("notification: invalid JSON: %v", err)
		_ = d.Nack(false, false)
		return
	}

	if strings.TrimSpace(ev.EventID) == "" {
		log.Printf("notification: missing event_id")
		_ = d.Nack(false, false)
		return
	}

	if _, err := uuid.Parse(ev.EventID); err != nil {
		log.Printf("notification: invalid event_id: %v", err)
		_ = d.Nack(false, false)
		return
	}

	claimed, err := c.repo.TryClaim(ctx, ev.EventID)
	if err != nil {
		log.Printf("notification: idempotency store: %v", err)
		_ = d.Nack(false, true)
		return
	}

	if !claimed {
		_ = d.Ack(false)
		return
	}

	amountStr := fmt.Sprintf("$%.2f", float64(ev.Amount)/100)
	log.Printf("[Notification] Sent email to %s for Order #%s. Amount: %s", ev.CustomerEmail, ev.OrderID, amountStr)

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
