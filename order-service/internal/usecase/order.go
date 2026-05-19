package usecase

import (
	"context"
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

type OrderCache interface {
	Get(ctx context.Context, id string) (*domain.Order, bool, error)
	Set(ctx context.Context, order domain.Order) error
	Delete(ctx context.Context, id string) error
}

type PaymentClient interface {
	AuthorizePayment(ctx context.Context, orderID string, amount int64, customerEmail string) (string, string, error)
}

type OrderUseCase struct {
	orderRepo     OrderRepository
	paymentClient PaymentClient
	orderCache    OrderCache
}

func NewOrderUseCase(orderRepo OrderRepository, paymentClient PaymentClient, orderCache OrderCache) *OrderUseCase {
	return &OrderUseCase{
		orderRepo:     orderRepo,
		paymentClient: paymentClient,
		orderCache:    orderCache,
	}
}

func (uc *OrderUseCase) invalidateOrder(ctx context.Context, id string) {
	if uc.orderCache == nil {
		return
	}
	_ = uc.orderCache.Delete(ctx, id)
}

func (uc *OrderUseCase) GetRevenueByCustomerID(customerID string) (int64, int, error) {
	return uc.orderRepo.GetRevenueByCustomerID(customerID)
}

func (uc *OrderUseCase) CreateOrder(ctx context.Context, customerID string, itemName string, amount int64, customerEmail string, idempotencyKey string) (*domain.Order, error) {
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

	status, _, err := uc.paymentClient.AuthorizePayment(ctx, order.ID, order.Amount, customerEmail)
	if err != nil {
		uc.orderRepo.UpdateStatus(order.ID, "Failed")
		uc.invalidateOrder(ctx, order.ID)
		order.Status = "Failed"
		return &order, errors.New("payment service is not available")
	}

	if status == "Authorized" {
		uc.orderRepo.UpdateStatus(order.ID, "Paid")
		uc.invalidateOrder(ctx, order.ID)
		order.Status = "Paid"
	} else {
		uc.orderRepo.UpdateStatus(order.ID, "Failed")
		uc.invalidateOrder(ctx, order.ID)
		order.Status = "Failed"
	}

	return &order, nil
}

func (uc *OrderUseCase) GetOrder(ctx context.Context, id string) (*domain.Order, error) {
	if uc.orderCache != nil {
		cached, ok, err := uc.orderCache.Get(ctx, id)
		if err == nil && ok {
			return cached, nil
		}
	}

	order, err := uc.orderRepo.GetByID(id)
	if err != nil {
		return nil, err
	}

	if uc.orderCache != nil {
		_ = uc.orderCache.Set(ctx, *order)
	}

	return order, nil
}

func (uc *OrderUseCase) CancelOrder(ctx context.Context, id string) (*domain.Order, error) {
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
	uc.invalidateOrder(ctx, id)

	order.Status = "Cancelled"
	return order, nil
}
