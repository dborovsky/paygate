package payment

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrNotFound: no such payment, or it belongs to another merchant.
	ErrNotFound = errors.New("payment not found")
	// ErrKeyReused: the idempotency key was already used with a different request body.
	ErrKeyReused = errors.New("idempotency key reused with a different request")
)

type Payment struct {
	ID         string
	MerchantID int64
	Amount     int64 // minor units: 1050 = 10.50 USD
	Currency   string
	Status     string
	CreatedAt  time.Time
}

type CreateParams struct {
	MerchantID     int64
	IdempotencyKey string
	RequestHash    []byte
	Amount         int64
	Currency       string
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const columns = `id::text, merchant_id, amount, currency, status, created_at`

func (r *Repository) Create(ctx context.Context, p CreateParams) (Payment, bool, error) {
	var pay Payment

	err := r.pool.QueryRow(ctx, `
	INSERT INTO payments (merchant_id, idempotency_key, request_hash, amount, currency)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (merchant_id, idempotency_key) DO NOTHING
	RETURNING `+columns,
		p.MerchantID, p.IdempotencyKey, p.RequestHash, p.Amount, p.Currency,
	).Scan(&pay.ID, &pay.MerchantID, &pay.Amount, &pay.Currency, &pay.Status, &pay.CreatedAt)

	if err == nil {
		return pay, true, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return Payment{}, false, fmt.Errorf("insert payment: %w", err)
	}

	var existingHash []byte
	err = r.pool.QueryRow(ctx, `
	       SELECT `+columns+`, request_hash
	       FROM payments
	       WHERE merchant_id = $1 AND idempotency_key = $2`,
		p.MerchantID, p.IdempotencyKey,
	).Scan(&pay.ID, &pay.MerchantID, &pay.Amount, &pay.Currency, &pay.Status, &pay.CreatedAt, &existingHash)
	if err != nil {
		return Payment{}, false, fmt.Errorf("select existing payment: %w", err)
	}

	if bytes.Equal(existingHash, p.RequestHash) {
		return pay, false, nil
	}

	return Payment{}, false, ErrKeyReused
}

// GetByID returns a payment only if it belongs to merchantID.
func (r *Repository) GetByID(ctx context.Context, merchantID int64, id string) (Payment, error) {
	pay := Payment{}
	err := r.pool.QueryRow(ctx, `
	SELECT `+columns+`
	FROM payments
	WHERE id = $1 AND merchant_id = $2`,
		id, merchantID).Scan(&pay.ID, &pay.MerchantID, &pay.Amount, &pay.Currency, &pay.Status, &pay.CreatedAt)

	if err == nil {
		return pay, nil
	}
	var pgErr *pgconn.PgError
	if errors.Is(err, pgx.ErrNoRows) || (errors.As(err, &pgErr) && pgErr.Code == "22P02") {
		return Payment{}, ErrNotFound
	}

	return Payment{}, fmt.Errorf("get payment: %w", err)
}
