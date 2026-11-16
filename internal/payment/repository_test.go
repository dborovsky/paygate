package payment

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dborovsky/paygate/internal/apikey"
	"github.com/dborovsky/paygate/internal/merchant"
)

const testDBURL = "postgres://paygate:paygate@localhost:5432/paygate?sslmode=disable"

func setup(t *testing.T) (*Repository, merchant.Merchant) {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), testDBURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if err := pool.Ping(t.Context()); err != nil {
		t.Skipf("postgres is not available, skipping: %v", err)
	}
	t.Cleanup(pool.Close)

	return NewRepository(pool), newTestMerchant(t, pool)
}

// Every test gets its own merchant, so "key-1" never collides between tests.
func newTestMerchant(t *testing.T, pool *pgxpool.Pool) merchant.Merchant {
	t.Helper()
	_, prefix, hash, err := apikey.Generate()
	if err != nil {
		t.Fatal(err)
	}
	m, err := merchant.NewRepository(pool).CreateWithKey(t.Context(), "Payment Test Shop", prefix, hash)
	if err != nil {
		t.Fatalf("create merchant: %v", err)
	}
	return m
}

func params(merchantID int64, key string, amount int64) CreateParams {
	return CreateParams{
		MerchantID:     merchantID,
		IdempotencyKey: key,
		RequestHash:    apikey.Hash(fmt.Sprintf("%s:%d", key, amount)),
		Amount:         amount,
		Currency:       "USD",
	}
}

func TestCreateNewPayment(t *testing.T) {
	repo, m := setup(t)

	pay, created, err := repo.Create(t.Context(), params(m.ID, "key-1", 1050))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !created {
		t.Error("created = false, want true for a new key")
	}
	if pay.ID == "" || pay.Status != "pending" || pay.Amount != 1050 {
		t.Errorf("unexpected payment: %+v", pay)
	}
}

func TestCreateSameKeyReturnsSamePayment(t *testing.T) {
	repo, m := setup(t)
	p := params(m.ID, "key-1", 1050)

	first, _, err := repo.Create(t.Context(), p)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	second, created, err := repo.Create(t.Context(), p)
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if created {
		t.Error("created = true, want false for a repeated key")
	}
	if second.ID != first.ID {
		t.Errorf("second ID = %s, want the same ID %s", second.ID, first.ID)
	}
}

func TestCreateSameKeyDifferentRequest(t *testing.T) {
	repo, m := setup(t)

	if _, _, err := repo.Create(t.Context(), params(m.ID, "key-1", 1050)); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, _, err := repo.Create(t.Context(), params(m.ID, "key-1", 9999))
	if !errors.Is(err, ErrKeyReused) {
		t.Errorf("err = %v, want ErrKeyReused", err)
	}
}

func TestSameKeyDifferentMerchants(t *testing.T) {
	repo, m1 := setup(t)
	m2 := newTestMerchant(t, repo.pool)

	pay1, _, err := repo.Create(t.Context(), params(m1.ID, "key-1", 1050))
	if err != nil {
		t.Fatal(err)
	}
	pay2, created, err := repo.Create(t.Context(), params(m2.ID, "key-1", 1050))
	if err != nil {
		t.Fatal(err)
	}
	if !created || pay1.ID == pay2.ID {
		t.Error("two merchants must get two different payments for the same key")
	}
}

func TestGetByIDOtherMerchant(t *testing.T) {
	repo, m1 := setup(t)
	m2 := newTestMerchant(t, repo.pool)

	pay, _, err := repo.Create(t.Context(), params(m1.ID, "key-1", 1050))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetByID(t.Context(), m1.ID, pay.ID); err != nil {
		t.Errorf("owner: GetByID: %v", err)
	}
	if _, err := repo.GetByID(t.Context(), m2.ID, pay.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("other merchant: err = %v, want ErrNotFound", err)
	}
}

func TestGetByIDInvalidID(t *testing.T) {
	repo, m := setup(t)

	if _, err := repo.GetByID(t.Context(), m.ID, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
