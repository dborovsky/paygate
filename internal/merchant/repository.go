package merchant

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("merchant not found")

type Merchant struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// CreateWithKey creates a merchant and its first API key in ONE transaction:
// either both rows are saved, or none.
func (r *Repository) CreateWithKey(ctx context.Context, name, keyPrefix string, keyHash []byte) (Merchant, error) {
	m := Merchant{Name: name}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Merchant{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `INSERT INTO merchants (name) VALUES($1) RETURNING id, created_at`,
		name).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return Merchant{}, fmt.Errorf("insert merchant: %w", err)
	}

	sql := `INSERT INTO api_keys (merchant_id, key_prefix, key_hash) VALUES ($1, $2, $3)`
	_, err = tx.Exec(ctx, sql, m.ID, keyPrefix, keyHash)
	if err != nil {
		return Merchant{}, fmt.Errorf("insert api key: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Merchant{}, fmt.Errorf("commit tx: %w", err)
	}

	return m, nil
}

// FindByKeyHash returns the merchant that owns an ACTIVE (not revoked) key with this hash.
func (r *Repository) FindByKeyHash(ctx context.Context, keyHash []byte) (Merchant, error) {
	m := Merchant{}
	sql := `SELECT m.id, m.name, m.created_at
	   FROM api_keys k
	   JOIN merchants m ON m.id = k.merchant_id
	   WHERE k.key_hash = $1 AND k.revoked_at IS NULL`
	err := r.pool.QueryRow(ctx, sql,
		keyHash).Scan(&m.ID, &m.Name, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Merchant{}, ErrNotFound
		}
		return Merchant{}, fmt.Errorf("find merchant by key: %w", err)
	}

	return m, nil
}
