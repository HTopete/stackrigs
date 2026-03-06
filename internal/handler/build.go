package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/htopete/stackrigs/internal/model"
	"github.com/htopete/stackrigs/internal/store"
)

type BuildHandler struct {
	store  *store.BuildStore
	logger *slog.Logger
}

func NewBuildHandler(s *store.BuildStore, logger *slog.Logger) *BuildHandler {
	return &BuildHandler{store: s, logger: logger}
}

func (h *BuildHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	if limit <= 0 {
		limit = 20
	}

	params := model.BuildListParams{
		Tech:   q.Get("tech"),
		Status: q.Get("status"),
		Sort:   q.Get("sort"),
		Limit:  limit,
		Offset: offset,
	}

	builds, total, err := h.store.List(params)
	if err != nil {
		h.logger.Error("failed to list builds", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, model.PaginatedResponse{
		Data:    builds,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: offset+limit < total,
	})
}

func (h *BuildHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid build id")
		return
	}

	build, err := h.store.GetByID(id)
	if err != nil {
		h.logger.Error("failed to get build", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if build == nil {
		writeError(w, http.StatusNotFound, "build not found")
		return
	}

	writeJSON(w, http.StatusOK, build)
}

func (h *BuildHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.BuilderID == 0 {
		writeError(w, http.StatusBadRequest, "name and builder_id are required")
		return
	}

	if req.Status == "" {
		req.Status = "building"
	}

	validStatuses := map[string]bool{"building": true, "launched": true, "abandoned": true, "paused": true}
	if !validStatuses[req.Status] {
		writeError(w, http.StatusBadRequest, "status must be one of: building, launched, paused, abandoned")
		return
	}

	build, err := h.store.Create(req)
	if err != nil {
		h.logger.Error("failed to create build", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, build)
}

func (h *BuildHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid build id")
		return
	}

	existing, err := h.store.GetByID(id)
	if err != nil {
		h.logger.Error("failed to get build for update", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "build not found")
		return
	}

	// In a real app, check auth here: r.Context().Value("builder_id") == existing.BuilderID

	var req model.UpdateBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status != nil {
		validStatuses := map[string]bool{"building": true, "launched": true, "abandoned": true, "paused": true}
		if !validStatuses[*req.Status] {
			writeError(w, http.StatusBadRequest, "status must be one of: building, launched, paused, abandoned")
			return
		}
	}

	build, err := h.store.Update(id, req)
	if err != nil {
		h.logger.Error("failed to update build", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, build)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(model.ErrorResponse{
		Error: message,
		Code:  status,
	})
}
