package main

import (
	"context"
	"io"
	"log"
	"os"

	apiv1 "github.com/shaminabd/ap2-contracts-go/apiv1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: subscribe <order_id>")
	}
	orderID := os.Args[1]

	addr := os.Getenv("ORDER_GRPC_ADDR")
	if addr == "" {
		addr = "localhost:50051"
	}

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := apiv1.NewOrderUpdateServiceClient(conn)
	stream, err := client.SubscribeToOrderUpdates(context.Background(), &apiv1.OrderRequest{OrderId: orderID})
	if err != nil {
		log.Fatal(err)
	}

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("order=%s status=%s updated_at=%s", msg.GetOrderId(), msg.GetStatus(), msg.GetUpdatedAt().AsTime())
	}
}
