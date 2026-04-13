package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"order-service/internal/usecase"
)

type OrderHandler struct {
	useCase *usecase.OrderUseCase
}

func NewOrderHandler(useCase *usecase.OrderUseCase) *OrderHandler {
	return &OrderHandler{useCase: useCase}
}

type createOrderRequest struct {
	CustomerID string `json:"customer_id"`
	ItemName   string `json:"item_name"`
	Amount     int64  `json:"amount"`
}

type orderResponse struct {
	ID         string `json:"id"`
	CustomerID string `json:"customer_id"`
	ItemName   string `json:"item_name"`
	Amount     int64  `json:"amount"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

type revenueResponse struct {
	CustomerID  string `json:"customer_id"`
	TotalAmount int64  `json:"total_amount"`
	OrdersCount int    `json:"orders_count"`
}

func (h *OrderHandler) CreateOrder(ctx *gin.Context) {
	var req createOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	idempotencyKey := ctx.GetHeader("Idempotency-Key")

	order, err := h.useCase.CreateOrder(req.CustomerID, req.ItemName, req.Amount, idempotencyKey)
	if err != nil {
		if strings.Contains(err.Error(), "not available") {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, orderResponse{
		ID:         order.ID,
		CustomerID: order.CustomerID,
		ItemName:   order.ItemName,
		Amount:     order.Amount,
		Status:     order.Status,
		CreatedAt:  order.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *OrderHandler) GetOrder(ctx *gin.Context) {
	id := ctx.Param("id")

	order, err := h.useCase.GetOrder(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	ctx.JSON(http.StatusOK, orderResponse{
		ID:         order.ID,
		CustomerID: order.CustomerID,
		ItemName:   order.ItemName,
		Amount:     order.Amount,
		Status:     order.Status,
		CreatedAt:  order.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *OrderHandler) CancelOrder(ctx *gin.Context) {
	id := ctx.Param("id")

	order, err := h.useCase.CancelOrder(id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, orderResponse{
		ID:         order.ID,
		CustomerID: order.CustomerID,
		ItemName:   order.ItemName,
		Amount:     order.Amount,
		Status:     order.Status,
		CreatedAt:  order.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *OrderHandler) GetRevenueByCustomerID(ctx *gin.Context) {
	customerID := ctx.Query("customer_id")
	if customerID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "customer_id is required"})
		return
	}

	total, count, err := h.useCase.GetRevenueByCustomerID(customerID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
	}

	ctx.JSON(http.StatusOK, revenueResponse{
		CustomerID:  customerID,
		TotalAmount: total,
		OrdersCount: count,
	})
}

func (h *OrderHandler) RegisterRoutes(router *gin.Engine) {
	router.POST("/orders", h.CreateOrder)
	router.GET("/orders/revenue", h.GetRevenueByCustomerID)
	router.GET("/orders/:id", h.GetOrder)
	router.PATCH("/orders/:id/cancel", h.CancelOrder)
}
