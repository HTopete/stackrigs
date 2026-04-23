package handler

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/htopete/stackrigs/internal/middleware"
	"github.com/htopete/stackrigs/internal/store"
)

const avatarMaxUpload = 512 << 10  // 512 KB — client sends pre-optimised WebP
const coverMaxUpload = 2 << 20     // 2 MB

type UploadHandler struct {
	builderStore *store.BuilderStore
	buildStore   *store.BuildStore
	uploadDir    string
	baseURL      string
	logger       *slog.Logger
}

func NewUploadHandler(builderStore *store.BuilderStore, buildStore *store.BuildStore, uploadDir, baseURL string, logger *slog.Logger) *UploadHandler {
	return &UploadHandler{
		builderStore: builderStore,
		buildStore:   buildStore,
		uploadDir:    uploadDir,
		baseURL:      baseURL,
		logger:       logger,
	}
}

// UploadAvatar handles avatar uploads. The client is expected to resize and
// encode the image as WebP before uploading (Canvas API), so the server
// only validates the content type and persists the file as-is.
func (h *UploadHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, avatarMaxUpload)

	if err := r.ParseMultipartForm(avatarMaxUpload); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 512 KB)")
		return
	}

	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing avatar file")
		return
	}
	defer file.Close()

	// Read first 512 bytes to sniff content type
	header := make([]byte, 512)
	n, _ := file.Read(header)
	ct := http.DetectContentType(header[:n])

	// Accept WebP (client-optimised) or JPEG/PNG as fallback
	var ext string
	switch {
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	case strings.HasPrefix(ct, "image/jpeg"):
		ext = ".jpg"
	case strings.HasPrefix(ct, "image/png"):
		ext = ".png"
	default:
		writeError(w, http.StatusBadRequest, "only WebP, JPEG, and PNG images are allowed")
		return
	}

	// Reset reader to start
	if seeker, ok := file.(io.ReadSeeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	avatarDir := filepath.Join(h.uploadDir, "avatars")
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		h.logger.Error("failed to create avatar directory", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	filename := fmt.Sprintf("%s-%d%s", builder.Handle, time.Now().UnixMilli(), ext)
	destPath := filepath.Join(avatarDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		h.logger.Error("failed to create avatar file", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		h.logger.Error("failed to write avatar file", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	avatarURL := fmt.Sprintf("%s/uploads/avatars/%s", h.baseURL, filename)

	if _, err = h.builderStore.UpdateAvatar(builder.ID, avatarURL); err != nil {
		h.logger.Error("failed to update builder avatar", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	h.logger.Info("avatar uploaded", "builder", builder.Handle, "file", filename, "type", ext)
	writeJSON(w, http.StatusOK, map[string]string{"url": avatarURL})
}

// UploadCover handles cover image uploads for builds. The client resizes and
// encodes as WebP before uploading (Canvas API). Max 2 MB.
// PUT /api/upload/cover/{buildId}
func (h *UploadHandler) UploadCover(w http.ResponseWriter, r *http.Request) {
	builder := middleware.BuilderFromContext(r.Context())
	if builder == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	buildIDStr := chi.URLParam(r, "buildId")
	buildID, err := strconv.ParseInt(buildIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid build id")
		return
	}

	ownerID, err := h.buildStore.GetOwnerID(buildID)
	if err != nil {
		writeError(w, http.StatusNotFound, "build not found")
		return
	}
	if ownerID != builder.ID {
		writeError(w, http.StatusForbidden, "you can only upload covers for your own builds")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, coverMaxUpload)
	if err := r.ParseMultipartForm(coverMaxUpload); err != nil {
		writeError(w, http.StatusBadRequest, "file too large (max 2 MB)")
		return
	}

	file, _, err := r.FormFile("cover")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing cover file")
		return
	}
	defer file.Close()

	header := make([]byte, 512)
	n, _ := file.Read(header)
	ct := http.DetectContentType(header[:n])

	var ext string
	switch {
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	case strings.HasPrefix(ct, "image/jpeg"):
		ext = ".jpg"
	case strings.HasPrefix(ct, "image/png"):
		ext = ".png"
	default:
		writeError(w, http.StatusBadRequest, "only WebP, JPEG, and PNG images are allowed")
		return
	}

	if seeker, ok := file.(io.ReadSeeker); ok {
		if _, err := seeker.Seek(0, io.SeekStart); err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	coverDir := filepath.Join(h.uploadDir, "covers")
	if err := os.MkdirAll(coverDir, 0755); err != nil {
		h.logger.Error("failed to create cover directory", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	filename := fmt.Sprintf("%d-%d%s", buildID, time.Now().UnixMilli(), ext)
	destPath := filepath.Join(coverDir, filename)

	dst, err := os.Create(destPath)
	if err != nil {
		h.logger.Error("failed to create cover file", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		h.logger.Error("failed to write cover file", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	coverURL := fmt.Sprintf("%s/uploads/covers/%s", h.baseURL, filename)

	if err := h.buildStore.UpdateCoverImage(buildID, coverURL); err != nil {
		h.logger.Error("failed to update build cover", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	h.logger.Info("cover uploaded", "build_id", buildID, "file", filename)
	writeJSON(w, http.StatusOK, map[string]string{"url": coverURL})
}

// ServeUploads serves uploaded files with immutable cache headers.
func (h *UploadHandler) ServeUploads(w http.ResponseWriter, r *http.Request) {
	filePath := strings.TrimPrefix(r.URL.Path, "/uploads/")
	fullPath := filepath.Join(h.uploadDir, filePath)

	// Prevent directory traversal — require trailing separator so
	// /uploads_evil/file doesn't pass a prefix check against /uploads.
	absUpload, _ := filepath.Abs(h.uploadDir)
	absFile, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absFile, absUpload+string(filepath.Separator)) {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, fullPath)
}
