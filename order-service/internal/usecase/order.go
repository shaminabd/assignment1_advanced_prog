package usecase

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"order-service/internal/domain"
)

type OrderRepository interface {
	Save(order domain.Order) error
	GetByID(id string) (*domain.Order, error)
	UpdateStatus(id string, status string) error
	GetByIdempotencyKey(key string) (*domain.Order, error)
	GetRevenueByCustomerID(customerID string) (int64, int, error)
}

type PaymentClient interface {
	AuthorizePayment(orderID string, amount int64) (string, string, error)
}

type OrderUseCase struct {
	orderRepo     OrderRepository
	paymentClient PaymentClient
}

func NewOrderUseCase(orderRepo OrderRepository, paymentClient PaymentClient) *OrderUseCase {
	return &OrderUseCase{
		orderRepo:     orderRepo,
		paymentClient: paymentClient,
	}
}

func (uc *OrderUseCase) GetRevenueByCustomerID(customerID string) (int64, int, error) {
	return uc.orderRepo.GetRevenueByCustomerID(customerID)
}

func (uc *OrderUseCase) CreateOrder(customerID string, itemName string, amount int64, idempotencyKey string) (*domain.Order, error) {
	if amount <= 0 {
		return nil, errors.New("invalid amount")
	}

	if idempotencyKey != "" {
		existing, err := uc.orderRepo.GetByIdempotencyKey(idempotencyKey)
		if err == nil && existing != nil {
			return existing, nil
		}
	}

	order := domain.Order{
		ID:             uuid.New().String(),
		CustomerID:     customerID,
		ItemName:       itemName,
		Amount:         amount,
		Status:         "Pending",
		CreatedAt:      time.Now(),
		IdempotencyKey: idempotencyKey,
	}

	err := uc.orderRepo.Save(order)
	if err != nil {
		return nil, err
	}

	status, _, err := uc.paymentClient.AuthorizePayment(order.ID, order.Amount)
	if err != nil {
		uc.orderRepo.UpdateStatus(order.ID, "Failed")
		order.Status = "Failed"
		return &order, errors.New("payment service is not available")
	}

	if status == "Authorized" {
		uc.orderRepo.UpdateStatus(order.ID, "Paid")
		order.Status = "Paid"
	} else {
		uc.orderRepo.UpdateStatus(order.ID, "Failed")
		order.Status = "Failed"
	}

	return &order, nil
}

func (uc *OrderUseCase) GetOrder(id string) (*domain.Order, error) {
	return uc.orderRepo.GetByID(id)
}

func (uc *OrderUseCase) CancelOrder(id string) (*domain.Order, error) {
	order, err := uc.orderRepo.GetByID(id)
	if err != nil {
		return nil, errors.New("order not found")
	}

	if order.Status == "Paid" {
		return nil, errors.New("cannot cancel a paid order")
	}

	if order.Status != "Pending" {
		return nil, errors.New("order is not pending")
	}

	err = uc.orderRepo.UpdateStatus(id, "Cancelled")
	if err != nil {
		return nil, err
	}

	order.Status = "Cancelled"
	return order, nil
}
