package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	"notification-service/internal/event"
)

const (
	exchangeName = "payment.events"
	queueName    = "payment.completed"
	routingKey   = "payment.completed"
)

type Consumer struct {
	conn         *amqp.Connection
	ch           *amqp.Channel
	processedMu  sync.Mutex
	processedIDs map[string]struct{}
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

func NewConsumer(amqpURL string) (*Consumer, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, closeAndWrap(conn, nil, "rabbitmq channel: %w", err)
	}

	if err := ch.Qos(1, 0, false); err != nil {
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
		conn:         conn,
		ch:           ch,
		processedIDs: make(map[string]struct{}),
	}, nil
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
			c.handleDelivery(d)
		}
	}
}

func (c *Consumer) handleDelivery(d amqp.Delivery) {
	var ev event.PaymentCompleted
	_ = json.Unmarshal(d.Body, &ev)
	alreadyProcessed := c.isAlreadyProcessed(ev.OrderID)
	if alreadyProcessed {
		_ = d.Ack(false)
		return
	}

	amountStr := fmt.Sprintf("$%.2f", float64(ev.Amount)/100)
	log.Printf("[Notification] Sent email to %s for Order #%s. Amount: %s", ev.CustomerEmail, ev.OrderID, amountStr)

	if err := d.Ack(false); err != nil {
		log.Printf("notification: ack: %v", err)
	}
}

func (c *Consumer) isAlreadyProcessed(eventID string) bool {
	c.processedMu.Lock()
	defer c.processedMu.Unlock()

	if _, exists := c.processedIDs[eventID]; exists {
		return true
	}

	c.processedIDs[eventID] = struct{}{}
	return false
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
