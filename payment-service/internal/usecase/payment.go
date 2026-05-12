package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"payment-service/internal/domain"
	"payment-service/internal/messaging"
)

const defaultCustomerEmail = "user@example.com"

type PaymentRepository interface {
	Save(payment domain.Payment) error
	GetByOrderID(orderID string) (*domain.Payment, error)
}

type PaymentUseCase struct {
	repo PaymentRepository
	bus  messaging.PaymentCompletedPublisher
}

func NewPaymentUseCase(repo PaymentRepository, bus messaging.PaymentCompletedPublisher) *PaymentUseCase {
	return &PaymentUseCase{repo: repo, bus: bus}
}

func (uc *PaymentUseCase) AuthorizePayment(ctx context.Context, orderID string, amount int64, customerEmail string) (*domain.Payment, error) {
	if amount <= 0 {
		return nil, errors.New("invalid amount")
	}

	if strings.TrimSpace(customerEmail) == "" {
		customerEmail = defaultCustomerEmail
	}

	payment := domain.Payment{
		ID:            uuid.New().String(),
		OrderID:       orderID,
		TransactionID: uuid.New().String(),
		Amount:        amount,
		Status:        "Authorized",
		CustomerEmail: customerEmail,
	}

	if amount > 100000 {
		payment.Status = "Declined"
	}

	err := uc.repo.Save(payment)
	if err != nil {
		return nil, err
	}

	if uc.bus != nil && payment.Status == "Authorized" {
		ev := messaging.PaymentCompletedEvent{
			OrderID:       payment.OrderID,
			Amount:        payment.Amount,
			CustomerEmail: payment.CustomerEmail,
			Status:        payment.Status,
		}
		pubCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		if err := uc.bus.Publish(pubCtx, ev); err != nil {
			return nil, fmt.Errorf("publish payment event: %w", err)
		}
	}

	return &payment, nil
}

func (uc *PaymentUseCase) GetPaymentByOrderID(orderID string) (*domain.Payment, error) {
	return uc.repo.GetByOrderID(orderID)
}
