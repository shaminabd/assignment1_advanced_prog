package rabbitmq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"payment-service/internal/messaging"
)

const (
	exchangeName    = "payment.events"
	queueName       = "payment.completed"
	routingKey      = "payment.completed"
	dlxExchangeName = "payment.dlx"
	dlqQueueName    = "payment.completed.dlq"
	dlqRoutingKey   = "payment.completed.dlq"
)

type Publisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

func closeConnAndChannel(conn *amqp.Connection, ch *amqp.Channel) {
	if ch != nil {
		_ = ch.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}
}

func NewPublisher(amqpURL string) (*Publisher, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		closeConnAndChannel(conn, nil)
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
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
		closeConnAndChannel(conn, ch)
		return nil, fmt.Errorf("exchange declare: %w", err)
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
		closeConnAndChannel(conn, ch)
		return nil, fmt.Errorf("dlx exchange declare: %w", err)
	}

	if _, err := ch.QueueDeclare(
		dlqQueueName,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		closeConnAndChannel(conn, ch)
		return nil, fmt.Errorf("dlq queue declare: %w", err)
	}

	if err := ch.QueueBind(dlqQueueName, dlqRoutingKey, dlxExchangeName, false, nil); err != nil {
		closeConnAndChannel(conn, ch)
		return nil, fmt.Errorf("dlq bind: %w", err)
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
		closeConnAndChannel(conn, ch)
		return nil, fmt.Errorf("queue declare: %w", err)
	}

	if err := ch.QueueBind(queueName, routingKey, exchangeName, false, nil); err != nil {
		closeConnAndChannel(conn, ch)
		return nil, fmt.Errorf("queue bind: %w", err)
	}

	if err := ch.Confirm(false); err != nil {
		closeConnAndChannel(conn, ch)
		return nil, fmt.Errorf("confirm mode: %w", err)
	}

	return &Publisher{conn: conn, channel: ch}, nil
}

func (p *Publisher) Publish(ctx context.Context, e messaging.PaymentCompletedEvent) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}

	confirms := p.channel.NotifyPublish(make(chan amqp.Confirmation, 1))

	err = p.channel.PublishWithContext(ctx,
		exchangeName,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
	if err != nil {
		return err
	}

	select {
	case confirm := <-confirms:
		if !confirm.Ack {
			return errors.New("rabbitmq: broker did not ack publish")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Second):
		return errors.New("rabbitmq: publish confirm timeout")
	}
}

func (p *Publisher) Close() error {
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}
