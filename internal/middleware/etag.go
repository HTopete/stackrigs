package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// etagResponseWriter captures the response body so we can compute an ETag.
type etagResponseWriter struct {
	http.ResponseWriter
	buf        bytes.Buffer
	statusCode int
	written    bool
}

func (w *etagResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.written = true
}

func (w *etagResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.statusCode = http.StatusOK
		w.written = true
	}
	return w.buf.Write(b)
}

// ETag is middleware that generates an ETag header based on a SHA-256 hash of the
// response body. If the client sends an If-None-Match header that matches, it
// responds with 304 Not Modified. Only applies to GET requests.
func ETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only apply ETag logic to GET requests
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		// Wrap the response writer to capture the body
		ew := &etagResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(ew, r)

		body := ew.buf.Bytes()

		// Only generate ETags for successful responses
		if ew.statusCode < 200 || ew.statusCode >= 300 {
			w.WriteHeader(ew.statusCode)
			w.Write(body)
			return
		}

		// Compute ETag from response body
		hash := sha256.Sum256(body)
		etag := `"` + hex.EncodeToString(hash[:16]) + `"`

		// Check If-None-Match
		ifNoneMatch := r.Header.Get("If-None-Match")
		if ifNoneMatch != "" {
			// Support comma-separated ETags
			for _, candidate := range strings.Split(ifNoneMatch, ",") {
				candidate = strings.TrimSpace(candidate)
				if candidate == etag || candidate == "*" {
					w.Header().Set("ETag", etag)
					w.WriteHeader(http.StatusNotModified)
					return
				}
			}
		}

		// Write ETag header and the full response
		w.Header().Set("ETag", etag)
		w.WriteHeader(ew.statusCode)
		w.Write(body)
	})
}
