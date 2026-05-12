package messaging

import "context"

type PaymentCompletedEvent struct {
	OrderID       string `json:"order_id"`
	Amount        int64  `json:"amount"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
}

type PaymentCompletedPublisher interface {
	Publish(ctx context.Context, e PaymentCompletedEvent) error
	Close() error
}
