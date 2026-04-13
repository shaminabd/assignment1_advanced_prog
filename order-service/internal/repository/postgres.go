package repository

import (
	"database/sql"

	"order-service/internal/domain"
)

type StatusChangeFn func(orderID string, status string)

type PostgresOrderRepository struct {
	db             *sql.DB
	onStatusChange StatusChangeFn
}

func NewPostgresOrderRepository(db *sql.DB, onStatusChange StatusChangeFn) *PostgresOrderRepository {
	return &PostgresOrderRepository{db: db, onStatusChange: onStatusChange}
}

func (r *PostgresOrderRepository) Save(order domain.Order) error {
	_, err := r.db.Exec(
		"INSERT INTO orders (id, customer_id, item_name, amount, status, created_at, idempotency_key) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		order.ID, order.CustomerID, order.ItemName, order.Amount, order.Status, order.CreatedAt, nullIfEmpty(order.IdempotencyKey),
	)
	return err
}

func (r *PostgresOrderRepository) GetByID(id string) (*domain.Order, error) {
	row := r.db.QueryRow(
		"SELECT id, customer_id, item_name, amount, status, created_at FROM orders WHERE id = $1",
		id,
	)

	var order domain.Order
	err := row.Scan(&order.ID, &order.CustomerID, &order.ItemName, &order.Amount, &order.Status, &order.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *PostgresOrderRepository) UpdateStatus(id string, status string) error {
	_, err := r.db.Exec("UPDATE orders SET status = $1 WHERE id = $2", status, id)
	if err != nil {
		return err
	}
	if r.onStatusChange != nil {
		r.onStatusChange(id, status)
	}
	return nil
}

func (r *PostgresOrderRepository) GetByIdempotencyKey(key string) (*domain.Order, error) {
	row := r.db.QueryRow(
		"SELECT id, customer_id, item_name, amount, status, created_at FROM orders WHERE idempotency_key = $1",
		key,
	)

	var order domain.Order
	err := row.Scan(&order.ID, &order.CustomerID, &order.ItemName, &order.Amount, &order.Status, &order.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *PostgresOrderRepository) GetRevenueByCustomerID(customerID string) (int64, int, error) {
	row := r.db.QueryRow(
		"SELECT COALESCE(SUM(amount), 0),COUNT(*) FROM orders WHERE customer_id = $1 AND status='Paid'",
		customerID)
	var total int64
	var count int
	err := row.Scan(&total, &count)
	if err != nil {
		return 0, 0, err
	}
	return total, count, nil
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
