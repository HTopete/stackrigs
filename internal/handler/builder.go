package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/htopete/stackrigs/internal/middleware"
	"github.com/htopete/stackrigs/internal/model"
	"github.com/htopete/stackrigs/internal/store"
)

type BuilderHandler struct {
	store  *store.BuilderStore
	logger *slog.Logger
}

func NewBuilderHandler(s *store.BuilderStore, logger *slog.Logger) *BuilderHandler {
	return &BuilderHandler{store: s, logger: logger}
}

func (h *BuilderHandler) GetByHandle(w http.ResponseWriter, r *http.Request) {
	handle := chi.URLParam(r, "handle")
	if handle == "" {
		writeError(w, http.StatusBadRequest, "handle is required")
		return
	}

	builder, err := h.store.GetByHandle(handle)
	if err != nil {
		h.logger.Error("failed to get builder", "error", err, "handle", handle)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if builder == nil {
		writeError(w, http.StatusNotFound, "builder not found")
		return
	}

	writeJSON(w, http.StatusOK, builder)
}

func (h *BuilderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateBuilderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Handle == "" || req.DisplayName == "" || req.InvitationCode == "" {
		writeError(w, http.StatusBadRequest, "handle, display_name, and invitation_code are required")
		return
	}

	req.Handle = strings.ToLower(strings.TrimSpace(req.Handle))

	existing, err := h.store.GetByHandle(req.Handle)
	if err != nil {
		h.logger.Error("failed to check existing builder", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "handle already taken")
		return
	}

	builder, err := h.store.Create(req)
	if err != nil {
		if strings.Contains(err.Error(), "invitation code") {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		h.logger.Error("failed to create builder", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, builder)
}

func (h *BuilderHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
		Bio         string `json:"bio"`
		Website     string `json:"website"`
		TwitterURL  string `json:"twitter_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Keep existing values if not provided
	displayName := req.DisplayName
	if displayName == "" {
		displayName = builder.DisplayName
	}

	updated, err := h.store.Update(builder.ID, displayName, req.Bio, req.Website, req.TwitterURL)
	if err != nil {
		h.logger.Error("failed to update builder", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, updated)
}
