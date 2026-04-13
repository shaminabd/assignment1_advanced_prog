package app

import (
	"database/sql"
	"log"
	"net"
	"os"

	apiv1 "github.com/shaminabd/ap2-contracts-go/apiv1"
	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	googlegrpc "google.golang.org/grpc"

	"payment-service/internal/repository"
	handler "payment-service/internal/transport/http"
	paymentgrpc "payment-service/internal/transport/grpc"
	"payment-service/internal/usecase"
)

func Run() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/payment_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	paymentRepo := repository.NewPostgresPaymentRepository(db)
	paymentUseCase := usecase.NewPaymentUseCase(paymentRepo)
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
			log.Fatalf("gRPC server: %v", err)
		}
	}()

	log.Printf("payment HTTP listening on :%s", httpPort)
	log.Fatal(router.Run(":" + httpPort))
}
