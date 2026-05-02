package usecase

import (
	"errors"
	"strings"

	"github.com/google/uuid"

	"payment-service/internal/domain"
)

const defaultCustomerEmail = "user@example.com"

type PaymentRepository interface {
	Save(payment domain.Payment) error
	GetByOrderID(orderID string) (*domain.Payment, error)
}

type PaymentUseCase struct {
	repo PaymentRepository
}

func NewPaymentUseCase(repo PaymentRepository) *PaymentUseCase {
	return &PaymentUseCase{repo: repo}
}

func (uc *PaymentUseCase) AuthorizePayment(orderID string, amount int64, customerEmail string) (*domain.Payment, error) {
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

	return &payment, nil
}

func (uc *PaymentUseCase) GetPaymentByOrderID(orderID string) (*domain.Payment, error) {
	return uc.repo.GetByOrderID(orderID)
}
