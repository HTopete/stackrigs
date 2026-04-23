package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/htopete/stackrigs/internal/config"
	"github.com/htopete/stackrigs/internal/database"
	"github.com/htopete/stackrigs/internal/handler"
	"github.com/htopete/stackrigs/internal/middleware"
	"github.com/htopete/stackrigs/internal/store"
)

func main() {
	// Support -healthcheck flag for Docker HEALTHCHECK in scratch image
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		resp, err := http.Get("http://localhost:" + getEnvOrDefault("PORT", "8080") + "/health")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg := config.Load()

	var logger *slog.Logger
	if cfg.IsProd() {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	} else {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	slog.SetDefault(logger)

	logger.Info("starting stackrigs server", "env", cfg.Env, "port", cfg.Port)

	db, err := database.Open(cfg.DatabasePath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Stores
	builderStore := store.NewBuilderStore(db)
	buildStore := store.NewBuildStore(db)
	technologyStore := store.NewTechnologyStore(db)
	searchStore := store.NewSearchStore(db)
	authStore := store.NewAuthStore(db)

	// Rebuild FTS5 search index on startup
	if err := searchStore.RebuildIndex(); err != nil {
		logger.Warn("failed to rebuild search index", "error", err)
	}

	// Start periodic session cleanup (every 30 minutes)
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			cleaned, err := authStore.CleanExpiredSessions()
			if err != nil {
				logger.Error("session cleanup error", "error", err)
			} else if cleaned > 0 {
				logger.Info("cleaned expired sessions", "count", cleaned)
			}
		}
	}()

	// Handlers
	healthH := handler.NewHealthHandler(db)
	uptimeStore := store.NewUptimeStore(db)
	infraH := handler.NewInfraHandler(uptimeStore)
	builderH := handler.NewBuilderHandler(builderStore, logger)
	buildH := handler.NewBuildHandler(buildStore, searchStore, logger)
	technologyH := handler.NewTechnologyHandler(technologyStore, logger)
	searchH := handler.NewSearchHandler(searchStore, logger)
	badgeH := handler.NewBadgeHandler(builderStore, buildStore, logger)
	uploadDir := filepath.Join(filepath.Dir(cfg.DatabasePath), "uploads")
	uploadH := handler.NewUploadHandler(builderStore, buildStore, uploadDir, cfg.BaseURL, logger)

	infraH.StartUptimeTracker()

	authH, err := handler.NewAuthHandler(authStore, builderStore, cfg, logger)
	if err != nil {
		logger.Error("failed to initialize auth handler", "error", err)
		os.Exit(1)
	}

	// Auth middleware
	authMW := middleware.NewAuthMiddleware(authStore, builderStore, cfg)

	// Rate limiters
	readLimiter := middleware.ReadLimiter()
	writeLimiter := middleware.WriteLimiter()

	// Router
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.CORS(cfg.AllowedOrigins))
	r.Use(middleware.RequestLogger(logger))
	r.Use(infraH.RequestCounter)

	// Health (no rate limit)
	r.Get("/health", healthH.Health)

	// Badge routes (public, no auth required)
	r.Route("/badge", func(r chi.Router) {
		r.Use(readLimiter.Middleware)
		r.Use(middleware.ETag)
		r.Get("/{handle}.svg", badgeH.ProfileBadge)
		r.Get("/{handle}/{buildId}.svg", badgeH.BuildBadge)
	})

	// Serve uploaded files (avatars, etc.)
	r.Get("/uploads/*", uploadH.ServeUploads)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Auth routes (public)
		r.Route("/auth", func(r chi.Router) {
			// WebAuthn registration (requires auth — must already be logged in)
			r.With(authMW.RequireAuth).Post("/webauthn/register/begin", authH.BeginRegistration)
			r.With(authMW.RequireAuth).Post("/webauthn/register/finish", authH.FinishRegistration)

			// WebAuthn login (public)
			r.Post("/webauthn/login/begin", authH.BeginLogin)
			r.Post("/webauthn/login/finish", authH.FinishLogin)

			// GitHub OAuth (public)
			r.Get("/github", authH.GitHubRedirect)
			r.Get("/github/callback", authH.GitHubCallback)

			// Session management
			r.Post("/logout", authH.Logout)
			r.With(authMW.RequireAuth).Get("/me", authH.Me)
		})

		// Read endpoints — 60 req/min, optional auth, ETag support
		r.Group(func(r chi.Router) {
			r.Use(readLimiter.Middleware)
			r.Use(authMW.OptionalAuth)
			r.Use(middleware.ETag)

			r.Get("/infra", infraH.Infra)
			r.Get("/infra/uptime", infraH.UptimeHistory)
			r.Get("/builders/{handle}", builderH.GetByHandle)
			r.Get("/builds", buildH.List)
			r.Get("/builds/{id}", buildH.GetByID)
			r.Get("/technologies", technologyH.List)
			r.Get("/technologies/{slug}", technologyH.GetBySlug)
			r.Get("/search", searchH.Search)
		})

		// SSE endpoint — separate group (no ETag, long-lived connection)
		r.Group(func(r chi.Router) {
			r.Use(readLimiter.Middleware)
			r.Use(authMW.OptionalAuth)
			r.Get("/infra/stream", infraH.InfraStream)
		})

		// Write endpoints — 10 req/min, auth required
		r.Group(func(r chi.Router) {
			r.Use(writeLimiter.Middleware)
			r.Use(authMW.RequireAuth)

			r.Post("/builders", builderH.Create)
			r.Put("/builders/me", builderH.UpdateProfile)
			r.Post("/builds", buildH.Create)
			r.Put("/builds/{id}", buildH.Update)
			r.Post("/builds/{id}/updates", buildH.CreateUpdate)
			r.Delete("/builds/{id}/updates/{updateId}", buildH.DeleteUpdate)
			r.Delete("/builds/{id}", buildH.Delete)
			r.Post("/upload/avatar", uploadH.UploadAvatar)
			r.Post("/upload/cover/{buildId}", uploadH.UploadCover)
		})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-done
	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped gracefully")
}

func getEnvOrDefault(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
