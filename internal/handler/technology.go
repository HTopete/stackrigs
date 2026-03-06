package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/htopete/stackrigs/internal/model"
	"github.com/htopete/stackrigs/internal/store"
)

type TechnologyHandler struct {
	store  *store.TechnologyStore
	logger *slog.Logger
}

func NewTechnologyHandler(s *store.TechnologyStore, logger *slog.Logger) *TechnologyHandler {
	return &TechnologyHandler{store: s, logger: logger}
}

func (h *TechnologyHandler) List(w http.ResponseWriter, r *http.Request) {
	techs, err := h.store.List()
	if err != nil {
		h.logger.Error("failed to list technologies", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, techs)
}

func (h *TechnologyHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	tech, err := h.store.GetBySlug(slug)
	if err != nil {
		h.logger.Error("failed to get technology", "error", err, "slug", slug)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if tech == nil {
		writeError(w, http.StatusNotFound, "technology not found")
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	if limit <= 0 {
		limit = 20
	}

	builds, total, err := h.store.GetBuildsBySlug(slug, limit, offset)
	if err != nil {
		h.logger.Error("failed to get builds for technology", "error", err, "slug", slug)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"technology": tech,
		"builds": model.PaginatedResponse{
			Data:    builds,
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasMore: offset+limit < total,
		},
	})
}
