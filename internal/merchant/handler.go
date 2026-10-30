package merchant

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dborovsky/paygate/internal/apikey"
)

type Handler struct {
	repo   *Repository
	logger *slog.Logger
}

func NewHandler(repo *Repository, logger *slog.Logger) *Handler {
	return &Handler{repo: repo, logger: logger}
}

type createRequest struct {
	Name string `json:"name"`
}

type createResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	APIKey    string    `json:"api_key"` // shown ONLY in this response, never again
}

// POST /merchants.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	name := strings.TrimSpace(req.Name)
	if len(name) > 200 {
		writeError(w, http.StatusBadRequest, "name is too long")
		return
	}

	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	key, prefix, hash, err := apikey.Generate()
	if err != nil {
		h.logger.Error("generate api key", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	m, err := h.repo.CreateWithKey(r.Context(), name, prefix, hash)
	if err != nil {
		h.logger.Error("create merchant", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusCreated, createResponse{
		Name:      name,
		ID:        m.ID,
		CreatedAt: m.CreatedAt,
		APIKey:    key,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

type meResponse struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	m, ok := FromContext(r.Context())
	if ok == false {
		h.logger.Error("merchant missing in context")
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, meResponse{ID: m.ID, Name: m.Name, CreatedAt: m.CreatedAt})
}
