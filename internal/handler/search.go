package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/htopete/stackrigs/internal/store"
)

type SearchHandler struct {
	store  *store.SearchStore
	logger *slog.Logger
}

func NewSearchHandler(s *store.SearchStore, logger *slog.Logger) *SearchHandler {
	return &SearchHandler{store: s, logger: logger}
}

func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "query parameter 'q' is required")
		return
	}

	if len(q) > 100 {
		q = q[:100]
	}

	results, err := h.store.Search(q)
	if err != nil {
		h.logger.Error("search failed", "error", err, "query", q)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, results)
}
