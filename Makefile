# StackRigs — Development & Operations Makefile
# Works on ARM64 (Pi 5) and AMD64 (VPS/dev machine).

.PHONY: help dev dev-backend dev-frontend dev-docker build build-backend build-frontend \
        deploy deploy-backend deploy-frontend test lint backup migrate clean

# Default target
help:
	@echo "StackRigs — Available targets:"
	@echo ""
	@echo "  Development:"
	@echo "    make dev              Run backend + frontend locally (parallel)"
	@echo "    make dev-backend      Run Go backend with hot-reload"
	@echo "    make dev-frontend     Run Astro frontend dev server"
	@echo "    make dev-docker       Run everything via Docker Compose (dev mode)"
	@echo ""
	@echo "  Build:"
	@echo "    make build            Build both backend and frontend"
	@echo "    make build-backend    Build Docker image for backend"
	@echo "    make build-frontend   Build Astro static output"
	@echo ""
	@echo "  Deploy:"
	@echo "    make deploy           Deploy backend (Pi/VPS) + frontend (Cloudflare Pages)"
	@echo "    make deploy-backend   Deploy backend only"
	@echo "    make deploy-frontend  Deploy frontend to Cloudflare Pages"
	@echo ""
	@echo "  Operations:"
	@echo "    make backup           Run SQLite hot backup (+ R2 upload if configured)"
	@echo "    make test             Run Go tests"
	@echo "    make lint             Run Go linter"
	@echo "    make migrate          Run database migrations"
	@echo "    make clean            Remove build artifacts and temp files"

# ---------------------------------------------------------------------------
# Development
# ---------------------------------------------------------------------------

dev-backend:
	go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

dev:
	@echo "Starting backend and frontend in parallel..."
	@echo "Backend: http://localhost:8080"
	@echo "Frontend: http://localhost:4321"
	$(MAKE) -j2 dev-backend dev-frontend

dev-docker:
	docker compose -f docker-compose.dev.yml up --build

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

build-backend:
	docker compose build

build-frontend:
	cd frontend && npm ci && npm run build

build: build-backend build-frontend

# ---------------------------------------------------------------------------
# Deploy
# ---------------------------------------------------------------------------

deploy-backend:
	./scripts/deploy.sh

deploy-frontend:
	./scripts/deploy-frontend.sh

deploy: deploy-backend deploy-frontend

# ---------------------------------------------------------------------------
# Test & Lint
# ---------------------------------------------------------------------------

test:
	go test ./... -v -count=1

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run ./...

# ---------------------------------------------------------------------------
# Operations
# ---------------------------------------------------------------------------

backup:
	./scripts/backup.sh

migrate:
	@echo "Applying schema to database..."
	sqlite3 $${DATABASE_PATH:-./data/stackrigs.db} < database/schema.sql

clean:
	rm -rf tmp/ frontend/dist/ frontend/.astro/ frontend/node_modules/.cache/
	docker compose down --remove-orphans 2>/dev/null || true
	docker image prune -f 2>/dev/null || true
