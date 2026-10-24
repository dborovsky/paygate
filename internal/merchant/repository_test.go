package merchant

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dborovsky/paygate/internal/apikey"
)

const testDBURL = "postgres://paygate:paygate@localhost:5432/paygate?sslmode=disable"

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), testDBURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	if err := pool.Ping(t.Context()); err != nil {
		t.Skipf("postgres is not available, skipping: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewRepository(pool)
}

func TestCreateAndFindByKeyHash(t *testing.T) {
	repo := newTestRepo(t)

	key, prefix, hash, err := apikey.Generate()
	if err != nil {
		t.Fatal(err)
	}

	created, err := repo.CreateWithKey(t.Context(), "Test Shop", prefix, hash)
	if err != nil {
		t.Fatalf("CreateWithKey: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected a non-zero merchant ID")
	}

	// the middleware will do exactly this: hash the incoming key and look it up
	found, err := repo.FindByKeyHash(t.Context(), apikey.Hash(key))
	if err != nil {
		t.Fatalf("FindByKeyHash: %v", err)
	}
	if found.ID != created.ID || found.Name != "Test Shop" {
		t.Errorf("found %+v, want %+v", found, created)
	}
}

func TestFindByKeyHashNotFound(t *testing.T) {
	repo := newTestRepo(t)

	_, err := repo.FindByKeyHash(t.Context(), apikey.Hash("sk_live_does_not_exist"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
