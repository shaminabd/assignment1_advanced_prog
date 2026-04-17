package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"payment-service/internal/usecase"
)

type PaymentHandler struct {
	useCase *usecase.PaymentUseCase
}

func NewPaymentHandler(useCase *usecase.PaymentUseCase) *PaymentHandler {
	return &PaymentHandler{useCase: useCase}
}

type createPaymentRequest struct {
	OrderID string `json:"order_id"`
	Amount  int64  `json:"amount"`
}

type paymentResponse struct {
	ID            string `json:"id"`
	OrderID       string `json:"order_id"`
	TransactionID string `json:"transaction_id"`
	Amount        int64  `json:"amount"`
	Status        string `json:"status"`
}

type listPaymentsResponse struct {
	Payments []paymentResponse `json:"payments"`
}

func (h *PaymentHandler) CreatePayment(ctx *gin.Context) {
	var req createPaymentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	payment, err := h.useCase.AuthorizePayment(req.OrderID, req.Amount)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, paymentResponse{
		ID:            payment.ID,
		OrderID:       payment.OrderID,
		TransactionID: payment.TransactionID,
		Amount:        payment.Amount,
		Status:        payment.Status,
	})
}

func (h *PaymentHandler) GetPayment(ctx *gin.Context) {
	orderID := ctx.Param("order_id")

	payment, err := h.useCase.GetPaymentByOrderID(orderID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}

	ctx.JSON(http.StatusOK, paymentResponse{
		ID:            payment.ID,
		OrderID:       payment.OrderID,
		TransactionID: payment.TransactionID,
		Amount:        payment.Amount,
		Status:        payment.Status,
	})
}

func (h *PaymentHandler) ListPayments(ctx *gin.Context) {
	minAmount, err := parseAmountQuery(ctx.Query("min_amount"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid min_amount"})
		return
	}

	maxAmount, err := parseAmountQuery(ctx.Query("max_amount"))
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid max_amount"})
		return
	}

	payments, err := h.useCase.ListPayments(minAmount, maxAmount)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response := make([]paymentResponse, 0, len(payments))
	for _, payment := range payments {
		response = append(response, paymentResponse{
			ID:            payment.ID,
			OrderID:       payment.OrderID,
			TransactionID: payment.TransactionID,
			Amount:        payment.Amount,
			Status:        payment.Status,
		})
	}

	ctx.JSON(http.StatusOK, listPaymentsResponse{Payments: response})
}

func parseAmountQuery(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func (h *PaymentHandler) RegisterRoutes(router *gin.Engine) {
	router.POST("/payments", h.CreatePayment)
	router.GET("/payments", h.ListPayments)
	router.GET("/payments/:order_id", h.GetPayment)
}
