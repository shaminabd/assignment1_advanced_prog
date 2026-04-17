package repository

import (
	"database/sql"
	"strconv"

	"payment-service/internal/domain"
)

type PostgresPaymentRepository struct {
	db *sql.DB
}

func NewPostgresPaymentRepository(db *sql.DB) *PostgresPaymentRepository {
	return &PostgresPaymentRepository{db: db}
}

func (r *PostgresPaymentRepository) Save(payment domain.Payment) error {
	_, err := r.db.Exec(
		"INSERT INTO payments (id, order_id, transaction_id, amount, status) VALUES ($1, $2, $3, $4, $5)",
		payment.ID, payment.OrderID, payment.TransactionID, payment.Amount, payment.Status,
	)
	return err
}

func (r *PostgresPaymentRepository) GetByOrderID(orderID string) (*domain.Payment, error) {
	row := r.db.QueryRow(
		"SELECT id, order_id, transaction_id, amount, status FROM payments WHERE order_id = $1",
		orderID,
	)

	var payment domain.Payment
	err := row.Scan(&payment.ID, &payment.OrderID, &payment.TransactionID, &payment.Amount, &payment.Status)
	if err != nil {
		return nil, err
	}

	return &payment, nil
}

func (r *PostgresPaymentRepository) FindByAmountRange(minAmount, maxAmount int64) ([]domain.Payment, error) {
	query := "SELECT id, order_id, transaction_id, amount, status FROM payments WHERE 1=1"
	args := make([]interface{}, 0, 2)
	argIndex := 1

	if minAmount > 0 {
		query += " AND amount >= $" + strconv.Itoa(argIndex)
		args = append(args, minAmount)
		argIndex++
	}

	if maxAmount > 0 {
		query += " AND amount <= $" + strconv.Itoa(argIndex)
		args = append(args, maxAmount)
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	payments := make([]domain.Payment, 0)
	for rows.Next() {
		var payment domain.Payment
		err = rows.Scan(&payment.ID, &payment.OrderID, &payment.TransactionID, &payment.Amount, &payment.Status)
		if err != nil {
			return nil, err
		}
		payments = append(payments, payment)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return payments, nil
}
