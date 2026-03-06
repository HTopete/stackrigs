package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/htopete/stackrigs/internal/model"
)

type InfraHandler struct {
	startTime    time.Time
	requestCount atomic.Int64
	cache        *infraCache
}

type infraCache struct {
	mu        sync.RWMutex
	metrics   *model.InfraMetrics
	expiresAt time.Time
}

func NewInfraHandler() *InfraHandler {
	return &InfraHandler{
		startTime: time.Now(),
		cache:     &infraCache{},
	}
}

func (h *InfraHandler) IncrementRequests() {
	h.requestCount.Add(1)
}

func (h *InfraHandler) RequestCounter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.IncrementRequests()
		next.ServeHTTP(w, r)
	})
}

// Infra returns current infrastructure metrics as JSON (REST endpoint).
func (h *InfraHandler) Infra(w http.ResponseWriter, r *http.Request) {
	h.cache.mu.RLock()
	if h.cache.metrics != nil && time.Now().Before(h.cache.expiresAt) {
		metrics := h.cache.metrics
		h.cache.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(metrics)
		return
	}
	h.cache.mu.RUnlock()

	metrics := h.collectMetrics()

	h.cache.mu.Lock()
	h.cache.metrics = metrics
	h.cache.expiresAt = time.Now().Add(30 * time.Second)
	h.cache.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metrics)
}

// InfraStream sends infrastructure metrics as Server-Sent Events every 5 seconds.
// GET /api/infra/stream
// Content-Type: text/event-stream
// Gracefully closes when the client disconnects.
func (h *InfraHandler) InfraStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Send an initial event immediately
	metrics := h.collectMetrics()
	data, _ := json.Marshal(metrics)
	_, _ = fmt.Fprintf(w, "event: metrics\ndata: %s\n\n", data)
	flusher.Flush()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return
		case <-ticker.C:
			metrics := h.collectMetrics()
			data, err := json.Marshal(metrics)
			if err != nil {
				continue
			}

			_, writeErr := fmt.Fprintf(w, "event: metrics\ndata: %s\n\n", data)
			if writeErr != nil {
				// Client likely disconnected
				return
			}
			flusher.Flush()
		}
	}
}

func (h *InfraHandler) collectMetrics() *model.InfraMetrics {
	uptime := time.Since(h.startTime)
	uptimeStr := formatDuration(uptime)

	memTotal, memAvailable, memUsedPct := readMemInfo()
	loadAvg := readLoadAvg()

	elapsed := time.Since(h.startTime).Minutes()
	var reqPerMin int64
	if elapsed > 0 {
		reqPerMin = int64(float64(h.requestCount.Load()) / elapsed)
	}

	return &model.InfraMetrics{
		Uptime:       uptimeStr,
		MemTotal:     memTotal,
		MemAvailable: memAvailable,
		MemUsedPct:   memUsedPct,
		LoadAvg:      loadAvg,
		RequestsMin:  reqPerMin,
		Timestamp:    time.Now().UTC().Format(time.RFC3339),
	}
}

func readMemInfo() (total, available string, usedPct float64) {
	if runtime.GOOS != "linux" {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		totalMB := float64(m.Sys) / 1024 / 1024
		allocMB := float64(m.Alloc) / 1024 / 1024
		availMB := totalMB - allocMB
		if totalMB > 0 {
			usedPct = (allocMB / totalMB) * 100
		}
		return fmt.Sprintf("%.1f MB (process)", totalMB), fmt.Sprintf("%.1f MB (process)", availMB), usedPct
	}

	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return "unknown", "unknown", 0
	}
	defer f.Close()

	var totalKB, availKB int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			totalKB = parseMemValue(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			availKB = parseMemValue(line)
		}
	}

	if totalKB > 0 {
		usedPct = float64(totalKB-availKB) / float64(totalKB) * 100
	}

	return fmt.Sprintf("%d MB", totalKB/1024), fmt.Sprintf("%d MB", availKB/1024), usedPct
}

func parseMemValue(line string) int64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		val, err := strconv.ParseInt(fields[1], 10, 64)
		if err == nil {
			return val
		}
	}
	return 0
}

func readLoadAvg() string {
	if runtime.GOOS != "linux" {
		return "N/A (non-linux)"
	}

	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "unknown"
	}
	parts := strings.Fields(string(data))
	if len(parts) >= 3 {
		return strings.Join(parts[:3], " ")
	}
	return string(data)
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
