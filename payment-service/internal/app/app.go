package app

import (
	"database/sql"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"

	"payment-service/internal/repository"
	handler "payment-service/internal/transport/http"
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}

	log.Fatal(router.Run(":" + port))
}
