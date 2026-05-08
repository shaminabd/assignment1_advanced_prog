package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"notification-service/internal/event"
	"notification-service/internal/repository"
)

const (
	exchangeName    = "payment.events"
	queueName       = "payment.completed"
	routingKey      = "payment.completed"
	dlxExchangeName = "payment.dlx"
	dlqQueueName    = "payment.completed.dlq"
	dlqRoutingKey   = "payment.completed.dlq"
)

type Consumer struct {
	conn            *amqp.Connection
	ch              *amqp.Channel
	repo            *repository.ProcessedRepository
	dlqDemoMu       sync.Mutex
	dlqDemoAttempts map[string]int
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

func NewConsumer(amqpURL string, repo *repository.ProcessedRepository) (*Consumer, error) {
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

	if err := ch.ExchangeDeclare(
		dlxExchangeName,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, closeAndWrap(conn, ch, "dlx exchange declare: %w", err)
	}

	if _, err := ch.QueueDeclare(
		dlqQueueName,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, closeAndWrap(conn, ch, "dlq queue declare: %w", err)
	}

	if err := ch.QueueBind(dlqQueueName, dlqRoutingKey, dlxExchangeName, false, nil); err != nil {
		return nil, closeAndWrap(conn, ch, "dlq bind: %w", err)
	}

	mainQueueArgs := amqp.Table{
		"x-dead-letter-exchange":    dlxExchangeName,
		"x-dead-letter-routing-key": dlqRoutingKey,
	}

	if _, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		mainQueueArgs,
	); err != nil {
		return nil, closeAndWrap(conn, ch, "queue declare: %w", err)
	}

	if err := ch.QueueBind(queueName, routingKey, exchangeName, false, nil); err != nil {
		return nil, closeAndWrap(conn, ch, "queue bind: %w", err)
	}

	return &Consumer{
		conn:            conn,
		ch:              ch,
		repo:            repo,
		dlqDemoAttempts: make(map[string]int),
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	dlqDeliveries, err := c.ch.Consume(dlqQueueName, "notification-dlq-monitor", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume dlq: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case d, ok := <-dlqDeliveries:
				if !ok {
					return
				}
				c.handleDLQDelivery(d)
			}
		}
	}()

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

func (c *Consumer) handleDLQDelivery(d amqp.Delivery) {
	log.Printf("[DLQ] message moved to dead-letter queue (exchange=%s routing=%s redelivered=%v): %s",
		d.Exchange, d.RoutingKey, d.Redelivered, string(d.Body))
	if err := d.Ack(false); err != nil {
		log.Printf("dlq ack: %v", err)
	}
}

func (c *Consumer) handleDelivery(ctx context.Context, d amqp.Delivery) {
	var ev event.PaymentCompleted
	if err := json.Unmarshal(d.Body, &ev); err != nil {
		log.Printf("notification: invalid JSON (drop): %v", err)
		_ = d.Ack(false)
		return
	}

	if strings.TrimSpace(ev.EventID) == "" {
		log.Printf("notification: missing event_id (drop)")
		_ = d.Ack(false)
		return
	}

	if _, err := uuid.Parse(ev.EventID); err != nil {
		log.Printf("notification: invalid event_id (drop): %v", err)
		_ = d.Ack(false)
		return
	}

	if strings.EqualFold(strings.TrimSpace(ev.CustomerEmail), event.DLQDemoEmail) {
		c.handleDLQDemo(d, ev)
		return
	}

	alreadyProcessed, err := c.repo.IsAlreadyProcessed(ctx, ev.EventID)
	if err != nil {
		log.Printf("notification: idempotency store: %v", err)
		_ = d.Nack(false, true)
		return
	}

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

func (c *Consumer) handleDLQDemo(d amqp.Delivery, ev event.PaymentCompleted) {
	c.dlqDemoMu.Lock()
	c.dlqDemoAttempts[ev.EventID]++
	n := c.dlqDemoAttempts[ev.EventID]
	c.dlqDemoMu.Unlock()

	log.Printf("notification: dlq demo — simulated failure (attempt %d/3) event_id=%s", n, ev.EventID)

	if n < 3 {
		if err := d.Nack(false, true); err != nil {
			log.Printf("notification: nack requeue: %v", err)
		}
		return
	}

	log.Printf("notification: dlq demo — max retries reached, rejecting to DLQ (event_id=%s)", ev.EventID)
	if err := d.Nack(false, false); err != nil {
		log.Printf("notification: nack dlq: %v", err)
	}

	c.dlqDemoMu.Lock()
	delete(c.dlqDemoAttempts, ev.EventID)
	c.dlqDemoMu.Unlock()
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
