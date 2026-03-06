package handler

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/htopete/stackrigs/internal/store"
)

// Editorial Zen palette
const (
	badgeBgLeft    = "#1a1a2e"
	badgeBgRight   = "#16213e"
	badgeFgLight   = "#e8e8e8"
	badgeAccent    = "#0f3460"
	badgeHighlight = "#e94560"
)

var badgeSVGTmpl = template.Must(template.New("badge").Parse(`<svg xmlns="http://www.w3.org/2000/svg" width="{{.Width}}" height="20" role="img" aria-label="{{.Label}}">
  <title>{{.Label}}</title>
  <linearGradient id="s" x2="0" y2="100%">
    <stop offset="0" stop-color="#555" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="r"><rect width="{{.Width}}" height="20" rx="3" fill="#fff"/></clipPath>
  <g clip-path="url(#r)">
    <rect width="{{.LeftWidth}}" height="20" fill="{{.BgLeft}}"/>
    <rect x="{{.LeftWidth}}" width="{{.RightWidth}}" height="20" fill="{{.BgRight}}"/>
    <rect width="{{.Width}}" height="20" fill="url(#s)"/>
  </g>
  <g fill="{{.FgColor}}" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="11">
    <text x="{{.LeftCenter}}" y="14">{{.LeftText}}</text>
    <text x="{{.RightCenter}}" y="14">{{.RightText}}</text>
  </g>
</svg>`))

type badgeData struct {
	Width       int
	LeftWidth   int
	RightWidth  int
	LeftCenter  float64
	RightCenter float64
	LeftText    string
	RightText   string
	Label       string
	BgLeft      string
	BgRight     string
	FgColor     string
}

// BadgeHandler generates SVG badges for builders and builds.
type BadgeHandler struct {
	builderStore *store.BuilderStore
	buildStore   *store.BuildStore
	logger       *slog.Logger
}

// NewBadgeHandler creates a new BadgeHandler.
func NewBadgeHandler(builderStore *store.BuilderStore, buildStore *store.BuildStore, logger *slog.Logger) *BadgeHandler {
	return &BadgeHandler{
		builderStore: builderStore,
		buildStore:   buildStore,
		logger:       logger,
	}
}

// ProfileBadge renders an SVG badge showing the builder's handle and build count.
// GET /badge/{handle}.svg
func (h *BadgeHandler) ProfileBadge(w http.ResponseWriter, r *http.Request) {
	handle := chi.URLParam(r, "handle")
	handle = strings.TrimSuffix(handle, ".svg")

	if handle == "" {
		http.NotFound(w, r)
		return
	}

	builder, err := h.builderStore.GetByHandle(handle)
	if err != nil {
		h.logger.Error("badge: failed to get builder", "error", err, "handle", handle)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if builder == nil {
		http.NotFound(w, r)
		return
	}

	// Count builds for this builder
	buildCount := h.countBuilderBuilds(builder.ID)

	leftText := builder.Handle
	rightText := fmt.Sprintf("%d builds", buildCount)
	if buildCount == 1 {
		rightText = "1 build"
	}

	data := h.makeBadge(leftText, rightText, badgeBgLeft, badgeAccent)

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Vary", "Accept-Encoding")

	if err := badgeSVGTmpl.Execute(w, data); err != nil {
		h.logger.Error("badge: template execution failed", "error", err)
	}
}

// BuildBadge renders an SVG badge for a specific build with its stack.
// GET /badge/{handle}/{buildId}.svg
func (h *BadgeHandler) BuildBadge(w http.ResponseWriter, r *http.Request) {
	handle := chi.URLParam(r, "handle")
	buildIDStr := chi.URLParam(r, "buildId")
	buildIDStr = strings.TrimSuffix(buildIDStr, ".svg")

	if handle == "" || buildIDStr == "" {
		http.NotFound(w, r)
		return
	}

	buildID, err := strconv.ParseInt(buildIDStr, 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	build, err := h.buildStore.GetByID(buildID)
	if err != nil {
		h.logger.Error("badge: failed to get build", "error", err, "id", buildID)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if build == nil {
		http.NotFound(w, r)
		return
	}

	// Verify the build belongs to the handle
	if build.Builder == nil || build.Builder.Handle != handle {
		http.NotFound(w, r)
		return
	}

	leftText := build.Name
	// Compose stack text from technologies
	techNames := make([]string, 0, len(build.Technologies))
	for _, t := range build.Technologies {
		techNames = append(techNames, t.Name)
	}
	rightText := strings.Join(techNames, " | ")
	if rightText == "" {
		rightText = build.Status
	}

	// Truncate if too long
	if len(rightText) > 60 {
		rightText = rightText[:57] + "..."
	}
	if len(leftText) > 40 {
		leftText = leftText[:37] + "..."
	}

	// Use status-based color for right side
	bgRight := badgeAccent
	switch build.Status {
	case "launched":
		bgRight = "#4A6741"
	case "building":
		bgRight = badgeHighlight
	case "paused":
		bgRight = "#8A8458"
	case "abandoned":
		bgRight = "#8A6858"
	}

	data := h.makeBadge(leftText, rightText, badgeBgLeft, bgRight)

	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Vary", "Accept-Encoding")

	if err := badgeSVGTmpl.Execute(w, data); err != nil {
		h.logger.Error("badge: template execution failed", "error", err)
	}
}

func (h *BadgeHandler) makeBadge(leftText, rightText, bgLeft, bgRight string) badgeData {
	leftWidth := textWidth(leftText) + 12
	rightWidth := textWidth(rightText) + 12
	totalWidth := leftWidth + rightWidth

	return badgeData{
		Width:       totalWidth,
		LeftWidth:   leftWidth,
		RightWidth:  rightWidth,
		LeftCenter:  float64(leftWidth) / 2,
		RightCenter: float64(leftWidth) + float64(rightWidth)/2,
		LeftText:    leftText,
		RightText:   rightText,
		Label:       fmt.Sprintf("%s: %s", leftText, rightText),
		BgLeft:      bgLeft,
		BgRight:     bgRight,
		FgColor:     badgeFgLight,
	}
}

// countBuilderBuilds returns the total number of builds for a builder.
func (h *BadgeHandler) countBuilderBuilds(builderID int64) int {
	count, err := h.buildStore.CountByBuilder(builderID)
	if err != nil {
		return 0
	}
	return count
}

// textWidth approximates the pixel width of a string for SVG rendering.
// Uses approximate character widths for Verdana 11px.
func textWidth(s string) int {
	width := 0
	for _, c := range s {
		switch {
		case c >= 'A' && c <= 'Z':
			width += 8
		case c >= 'a' && c <= 'z':
			width += 6
		case c >= '0' && c <= '9':
			width += 7
		case c == ' ':
			width += 4
		case c == '|':
			width += 4
		default:
			width += 7
		}
	}
	return width
}
