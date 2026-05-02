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

	"payment-service/internal/infrastructure/rabbitmq"
	"payment-service/internal/messaging"
	"payment-service/internal/repository"
	handler "payment-service/internal/transport/http"
	paymentgrpc "payment-service/internal/transport/grpc"
	"payment-service/internal/usecase"
)

func Run() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/payment_db?sslmode=disable"
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

	paymentRepo := repository.NewPostgresPaymentRepository(db)

	var pub messaging.PaymentCompletedPublisher
	if rabbitURL := os.Getenv("RABBITMQ_URL"); rabbitURL != "" {
		p, err := rabbitmq.NewPublisher(rabbitURL)
		if err != nil {
			log.Fatal(err)
		}
		pub = p
	}

	paymentUseCase := usecase.NewPaymentUseCase(paymentRepo, pub)
	paymentHandler := handler.NewPaymentHandler(paymentUseCase)

	router := gin.Default()
	paymentHandler.RegisterRoutes(router)

	httpPort := os.Getenv("PORT")
	if httpPort == "" {
		httpPort = "8084"
	}

	grpcListen := os.Getenv("PAYMENT_GRPC_LISTEN")
	if grpcListen == "" {
		grpcListen = ":50052"
	}

	lis, err := net.Listen("tcp", grpcListen)
	if err != nil {
		log.Fatal(err)
	}

	grpcServer := googlegrpc.NewServer(
		googlegrpc.ChainUnaryInterceptor(paymentgrpc.UnaryLoggingInterceptor),
	)
	apiv1.RegisterPaymentServiceServer(grpcServer, paymentgrpc.NewPaymentServer(paymentUseCase))

	go func() {
		log.Printf("payment gRPC listening on %s", grpcListen)
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server: %v", err)
		}
	}()

	httpSrv := &http.Server{
		Addr:    ":" + httpPort,
		Handler: router,
	}
	go func() {
		log.Printf("payment HTTP listening on :%s", httpPort)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("payment-service shutting down")

	grpcServer.GracefulStop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	if pub != nil {
		if err := pub.Close(); err != nil {
			log.Printf("rabbitmq close: %v", err)
		}
	}
}
