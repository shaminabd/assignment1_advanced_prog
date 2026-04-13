package grpc

import (
	apiv1 "github.com/shaminabd/ap2-contracts-go/apiv1"
	googlegrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"order-service/internal/domain"
	"order-service/internal/streaming"
)

type OrderReader interface {
	GetByID(id string) (*domain.Order, error)
}

type OrderUpdateServer struct {
	apiv1.UnimplementedOrderUpdateServiceServer
	repo OrderReader
	hub  *streaming.Hub
}

func NewOrderUpdateServer(repo OrderReader, hub *streaming.Hub) *OrderUpdateServer {
	return &OrderUpdateServer{repo: repo, hub: hub}
}

func (s *OrderUpdateServer) SubscribeToOrderUpdates(req *apiv1.OrderRequest, stream googlegrpc.ServerStreamingServer[apiv1.OrderStatusUpdate]) error {
	if req.GetOrderId() == "" {
		return status.Error(codes.InvalidArgument, "order_id is required")
	}

	order, err := s.repo.GetByID(req.GetOrderId())
	if err != nil {
		return status.Error(codes.NotFound, "order not found")
	}

	err = stream.Send(&apiv1.OrderStatusUpdate{
		OrderId:   order.ID,
		Status:    order.Status,
		UpdatedAt: timestamppb.New(order.CreatedAt),
	})
	if err != nil {
		return err
	}

	updates, unregister := s.hub.Register(req.GetOrderId())
	defer unregister()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case u, ok := <-updates:
			if !ok {
				return nil
			}
			if err := stream.Send(u); err != nil {
				return err
			}
		}
	}
}
