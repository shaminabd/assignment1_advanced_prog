package client

import (
	"context"
	"errors"
	"time"

	apiv1 "github.com/shaminabd/ap2-contracts-go/apiv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type GRPCPaymentClient struct {
	client apiv1.PaymentServiceClient
	conn   *grpc.ClientConn
}

func NewGRPCPaymentClient(ctx context.Context, addr string) (*GRPCPaymentClient, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(dialCtx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}

	return &GRPCPaymentClient{
		client: apiv1.NewPaymentServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *GRPCPaymentClient) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *GRPCPaymentClient) AuthorizePayment(orderID string, amount int64) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.ProcessPayment(ctx, &apiv1.PaymentRequest{
		OrderId: orderID,
		Amount:  amount,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok && (st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded) {
			return "", "", errors.New("payment service is not available")
		}
		return "", "", err
	}

	return resp.GetStatus(), resp.GetTransactionId(), nil
}
