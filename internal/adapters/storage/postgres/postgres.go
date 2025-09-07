package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"payment-processing-system/internal/core/domain"
)

//Repository is an implementation of the TransactionRepository port for PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new repository instance.
// Accepts a DSN (Data Source Name) to connect to.
func NewRepository(ctx context.Context, dsn string) (*Repository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	// Let's check that the connection to the database actually works.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	return &Repository{pool: pool}, nil
}

// Close closes the connection pool.
func (r *Repository) Close() {
	r.pool.Close()
}

//Save implements the TransactionRepository interface method.
func (r *Repository) Save(ctx context.Context, tx domain.Transaction) error {
	const sql = `
		INSERT INTO transactions 
		    (id, status, amount, currency, card_number_hash, idempotency_key, created_at, updated_at) 
		VALUES 
		    ($1, $2, $3, $4, $5, $6, $7, $7)
		ON CONFLICT (idempotency_key) DO NOTHING
	`
	_, err := r.pool.Exec(ctx, sql,
		tx.ID,
		tx.Status,
		tx.Amount,
		tx.Currency,
		tx.CardNumberHash,
		tx.IdempotencyKey,
		tx.CreatedAt,
		tx.CreatedAt, //TODO: updated_at = created_at для новой записи
	)

	if err != nil {
		// TODO: Здесь можно добавить проверку на specific postgres errors,
		// например, на нарушение unique constraint по idempotency_key.
		return fmt.Errorf("failed to save transaction: %w", err)
	}

	return nil
}
