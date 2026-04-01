package app

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"order-service/internal/client"
	"order-service/internal/repository"
	handler "order-service/internal/transport/http"
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

	paymentURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentURL == "" {
		paymentURL = "http://localhost:8084"
	}

	httpClient := &http.Client{
		Timeout: 2 * time.Second,
	}

	orderRepo := repository.NewPostgresOrderRepository(db)
	paymentClient := client.NewHTTPPaymentClient(paymentURL, httpClient)
	orderUseCase := usecase.NewOrderUseCase(orderRepo, paymentClient)
	orderHandler := handler.NewOrderHandler(orderUseCase)

	router := gin.Default()
	orderHandler.RegisterRoutes(router)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	log.Fatal(router.Run(":" + port))
}
