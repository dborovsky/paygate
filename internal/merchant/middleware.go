package merchant

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/dborovsky/paygate/internal/apikey"
)

// ctxKey is a private type, so no other package can read or overwrite our value by accident.
type ctxKey struct{}

// WithMerchant returns a copy of ctx that carries the merchant.
func WithMerchant(ctx context.Context, m Merchant) context.Context {
	return context.WithValue(ctx, ctxKey{}, m)
}

// FromContext returns the merchant put into ctx by RequireAPIKey.
// ok is false if there is no merchant (the middleware was not applied).
func FromContext(ctx context.Context) (Merchant, bool) {
	m, ok := ctx.Value(ctxKey{}).(Merchant)
	return m, ok
}

// RequireAPIKey checks the "Authorization: Bearer <key>" header.
// If the key is valid, it puts the merchant into the request context and calls next.
// Otherwise it answers 401 and next is NOT called.
func (h *Handler) RequireAPIKey(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if ok == false || key == "" {
			writeError(w, http.StatusUnauthorized, "missing api key")
			return
		}

		m, err := h.repo.FindByKeyHash(r.Context(), apikey.Hash(key))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "invalid api key")
				return
			}
			h.logger.Error("find merchant by api key", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		ctx := WithMerchant(r.Context(), m)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
