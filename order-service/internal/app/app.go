package app

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/order_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

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
	defer func() { _ = paymentClient.Close() }()

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
			log.Printf("gRPC server: %v", err)
		}
	}()

	httpSrv := &http.Server{
		Addr:    ":" + httpPort,
		Handler: router,
	}
	go func() {
		log.Printf("order HTTP listening on :%s", httpPort)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("order-service shutting down")

	grpcServer.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
}
