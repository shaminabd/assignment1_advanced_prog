package grpc

import (
	"context"

	apiv1 "github.com/shaminabd/ap2-contracts-go/apiv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"payment-service/internal/usecase"
)

type PaymentServer struct {
	apiv1.UnimplementedPaymentServiceServer
	uc *usecase.PaymentUseCase
}

func NewPaymentServer(uc *usecase.PaymentUseCase) *PaymentServer {
	return &PaymentServer{uc: uc}
}

func (s *PaymentServer) ProcessPayment(ctx context.Context, req *apiv1.PaymentRequest) (*apiv1.PaymentResponse, error) {
	if req.GetOrderId() == "" {
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}
	if req.GetAmount() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "amount must be positive")
	}

	payment, err := s.uc.AuthorizePayment(req.GetOrderId(), req.GetAmount())
	if err != nil {
		if err.Error() == "invalid amount" {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &apiv1.PaymentResponse{
		Id:            payment.ID,
		OrderId:       payment.OrderID,
		TransactionId: payment.TransactionID,
		Amount:        payment.Amount,
		Status:        payment.Status,
		CreatedAt:     timestamppb.Now(),
	}, nil
}
