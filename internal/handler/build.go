package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/htopete/stackrigs/internal/middleware"
	"github.com/htopete/stackrigs/internal/model"
	"github.com/htopete/stackrigs/internal/store"
)

type BuildHandler struct {
	store  *store.BuildStore
	search *store.SearchStore
	logger *slog.Logger
}

func NewBuildHandler(s *store.BuildStore, search *store.SearchStore, logger *slog.Logger) *BuildHandler {
	return &BuildHandler{store: s, search: search, logger: logger}
}

func (h *BuildHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	if limit <= 0 {
		limit = 20
	}

	params := model.BuildListParams{
		Techs:   q["tech"], // supports ?tech=go&tech=react
		Status:  q.Get("status"),
		Sort:    q.Get("sort"),
		Builder: q.Get("builder"),
		Limit:   limit,
		Offset:  offset,
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
	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req model.CreateBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.BuilderID = builder.ID

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
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

	if err := h.search.UpsertBuildIndex(build.ID, build.Name, build.Description); err != nil {
		h.logger.Warn("failed to index new build", "error", err, "id", build.ID)
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

	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
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

	if existing.BuilderID != builder.ID {
		writeError(w, http.StatusForbidden, "you can only edit your own builds")
		return
	}

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

	if err := h.search.UpsertBuildIndex(build.ID, build.Name, build.Description); err != nil {
		h.logger.Warn("failed to re-index updated build", "error", err, "id", build.ID)
	}

	writeJSON(w, http.StatusOK, build)
}

func (h *BuildHandler) CreateUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid build id")
		return
	}

	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	existing, err := h.store.GetByID(id)
	if err != nil {
		h.logger.Error("failed to get build", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "build not found")
		return
	}
	if existing.BuilderID != builder.ID {
		writeError(w, http.StatusForbidden, "you can only update your own builds")
		return
	}

	var req struct {
		Type    string `json:"type"`
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	validTypes := map[string]bool{
		"milestone": true, "stack_change": true, "infra_change": true,
		"tool_change": true, "reflection": true, "pivot": true,
	}
	if req.Type == "" {
		req.Type = "milestone"
	}
	if !validTypes[req.Type] {
		writeError(w, http.StatusBadRequest, "type must be one of: milestone, stack_change, infra_change, tool_change, reflection, pivot")
		return
	}

	update, err := h.store.CreateUpdate(id, req.Type, req.Title, req.Content)
	if err != nil {
		h.logger.Error("failed to create build update", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, update)
}

func (h *BuildHandler) DeleteUpdate(w http.ResponseWriter, r *http.Request) {
	buildIDStr := chi.URLParam(r, "id")
	buildID, err := strconv.ParseInt(buildIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid build id")
		return
	}

	updateIDStr := chi.URLParam(r, "updateId")
	updateID, err := strconv.ParseInt(updateIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid update id")
		return
	}

	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ownerID, err := h.store.GetOwnerID(buildID)
	if err != nil {
		writeError(w, http.StatusNotFound, "build not found")
		return
	}
	if ownerID != builder.ID {
		writeError(w, http.StatusForbidden, "you can only delete updates from your own builds")
		return
	}

	if err := h.store.DeleteUpdate(updateID, buildID); err != nil {
		writeError(w, http.StatusNotFound, "update not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *BuildHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid build id")
		return
	}

	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	ownerID, err := h.store.GetOwnerID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "build not found")
		return
	}
	if ownerID != builder.ID {
		writeError(w, http.StatusForbidden, "you can only delete your own builds")
		return
	}

	if err := h.store.Delete(id); err != nil {
		h.logger.Error("failed to delete build", "error", err, "id", id)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := h.search.DeleteIndex("build", id); err != nil {
		h.logger.Warn("failed to remove deleted build from index", "error", err, "id", id)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
