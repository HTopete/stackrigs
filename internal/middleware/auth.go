package middleware

import (
	"context"
	"net/http"

	"github.com/htopete/stackrigs/internal/config"
	"github.com/htopete/stackrigs/internal/model"
	"github.com/htopete/stackrigs/internal/store"
)

// authContextKey is an unexported type for auth context keys.
type authContextKey string

const (
	// BuilderCtxKey stores the authenticated builder in request context.
	BuilderCtxKey authContextKey = "builder"
	// SessionCookieName is the cookie name used for sessions.
	SessionCookieName = "stackrigs_session"
)

// AuthMiddleware provides RequireAuth and OptionalAuth middleware.
type AuthMiddleware struct {
	authStore    *store.AuthStore
	builderStore *store.BuilderStore
	cfg          *config.Config
}

// NewAuthMiddleware creates a new AuthMiddleware.
func NewAuthMiddleware(authStore *store.AuthStore, builderStore *store.BuilderStore, cfg *config.Config) *AuthMiddleware {
	return &AuthMiddleware{
		authStore:    authStore,
		builderStore: builderStore,
		cfg:          cfg,
	}
}

// RequireAuth is middleware that rejects unauthenticated requests with 401.
// It reads the session cookie, validates the session, fetches the builder,
// and injects the builder into the request context.
func (m *AuthMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		builder, err := m.extractBuilder(r)
		if err != nil || builder == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"authentication required","code":401}`))
			return
		}

		ctx := context.WithValue(r.Context(), BuilderCtxKey, builder)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OptionalAuth is middleware that injects the builder into context if authenticated,
// but allows the request to proceed even if not authenticated.
func (m *AuthMiddleware) OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		builder, _ := m.extractBuilder(r)
		if builder != nil {
			ctx := context.WithValue(r.Context(), BuilderCtxKey, builder)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// extractBuilder reads the session cookie, validates the session, and returns the builder.
func (m *AuthMiddleware) extractBuilder(r *http.Request) (*model.Builder, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return nil, err
	}

	if cookie.Value == "" {
		return nil, nil
	}

	session, err := m.authStore.GetSession(cookie.Value)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, nil
	}

	builder, err := m.builderStore.GetByID(session.BuilderID)
	if err != nil {
		return nil, err
	}

	return builder, nil
}

// BuilderFromContext extracts the authenticated builder from the request context.
func BuilderFromContext(ctx context.Context) *model.Builder {
	if b, ok := ctx.Value(BuilderCtxKey).(*model.Builder); ok {
		return b
	}
	return nil
}
