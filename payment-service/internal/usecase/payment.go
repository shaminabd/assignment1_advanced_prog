package usecase

import (
	"errors"

	"github.com/google/uuid"

	"payment-service/internal/domain"
)

type PaymentRepository interface {
	Save(payment domain.Payment) error
	GetByOrderID(orderID string) (*domain.Payment, error)
	FindByAmountRange(minAmount, maxAmount int64) ([]domain.Payment, error)
}

type PaymentUseCase struct {
	repo PaymentRepository
}

func NewPaymentUseCase(repo PaymentRepository) *PaymentUseCase {
	return &PaymentUseCase{repo: repo}
}

func (uc *PaymentUseCase) AuthorizePayment(orderID string, amount int64) (*domain.Payment, error) {
	if amount <= 0 {
		return nil, errors.New("invalid amount")
	}

	payment := domain.Payment{
		ID:            uuid.New().String(),
		OrderID:       orderID,
		TransactionID: uuid.New().String(),
		Amount:        amount,
		Status:        "Authorized",
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

func (uc *PaymentUseCase) ListPayments(minAmount, maxAmount int64) ([]domain.Payment, error) {
	if minAmount < 0 || maxAmount < 0 {
		return nil, errors.New("amount must not be negative")
	}

	if minAmount > 0 && maxAmount > 0 && minAmount > maxAmount {
		return nil, errors.New("min_amount must be less than or equal to max_amount")
	}

	return uc.repo.FindByAmountRange(minAmount, maxAmount)
}
