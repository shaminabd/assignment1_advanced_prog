package app

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"

	apiv1 "github.com/shaminabd/ap2-contracts-go/apiv1"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	googlegrpc "google.golang.org/grpc"

	"order-service/internal/client"
	"order-service/internal/repository"
	"order-service/internal/streaming"
	handler "order-service/internal/transport/http"
	ordergrpc "order-service/internal/transport/grpc"
	"order-service/internal/usecase"
)

func Run() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/order_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	hub := streaming.NewHub()
	orderRepo := repository.NewPostgresOrderRepository(db, hub.Notify)

	paymentAddr := os.Getenv("PAYMENT_GRPC_ADDR")
	if paymentAddr == "" {
		paymentAddr = "localhost:50052"
	}

	paymentClient, err := client.NewGRPCPaymentClient(context.Background(), paymentAddr)
	if err != nil {
		log.Fatalf("payment gRPC client: %v", err)
	}
	defer paymentClient.Close()

	orderUseCase := usecase.NewOrderUseCase(orderRepo, paymentClient)
	orderHandler := handler.NewOrderHandler(orderUseCase)

	router := gin.Default()
	orderHandler.RegisterRoutes(router)

	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8083"
	}

	orderGRPCListen := os.Getenv("ORDER_GRPC_LISTEN")
	if orderGRPCListen == "" {
		orderGRPCListen = ":50051"
	}

	lis, err := net.Listen("tcp", orderGRPCListen)
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := googlegrpc.NewServer()
	apiv1.RegisterOrderUpdateServiceServer(grpcServer, ordergrpc.NewOrderUpdateServer(orderRepo, hub))

	go func() {
		log.Printf("order gRPC listening on %s", orderGRPCListen)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("gRPC server: %v", err)
		}
	}()

	log.Printf("order HTTP listening on :%s", httpPort)
	log.Fatal(router.Run(":" + httpPort))
}
