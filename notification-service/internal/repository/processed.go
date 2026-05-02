package repository

import (
	"context"
	"database/sql"
)

type ProcessedRepository struct {
	db *sql.DB
}

func NewProcessedRepository(db *sql.DB) *ProcessedRepository {
	return &ProcessedRepository{db: db}
}

func (r *ProcessedRepository) TryClaim(ctx context.Context, eventID string) (claimed bool, err error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO processed_events (event_id) VALUES ($1::uuid) ON CONFLICT (event_id) DO NOTHING`,
		eventID,
	)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
