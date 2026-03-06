# StackRigs

An open index of what builders are actually building. Real stacks. Real infra. No noise.

**URL:** https://stackrigs.com
**Author:** Ruben Topete (@htopete_dev)
**License:** FSL-1.1-Apache-2.0 (source available, converts to Apache 2.0 after 2 years)

## Architecture

```
[Browser] → Cloudflare Edge
                ├── Cloudflare Pages (Astro 5.x static)
                └── Cloudflare Tunnel → Pi 5 / VPS (Go 1.24 API + SQLite)
```

- **Backend:** Go 1.24 + chi v5 + modernc.org/sqlite (pure Go, no CGO)
- **Frontend:** Astro 5.x (static output) + Preact islands
- **Database:** SQLite with WAL mode + FTS5 for search
- **Auth:** Passkeys (WebAuthn) + GitHub OAuth, session cookies (no JWT)
- **Infra:** Raspberry Pi 5 (16GB) in Mazatlan, Docker Compose, Cloudflare Tunnel
- **CDN:** Cloudflare Pages for static, Cloudflare free tier for API caching

## Project Structure

```
stackrigs/
├── cmd/server/main.go           # Go entry point (-healthcheck flag for Docker)
├── internal/
│   ├── config/                   # Env vars config
│   ├── database/                 # SQLite connection + migrations + FTS5
│   ├── handler/                  # HTTP handlers (auth, builds, badges, SSE, infra, health)
│   ├── middleware/               # CORS, rate limit, ETag, auth, logging
│   ├── model/                    # Go structs
│   └── store/                    # Database queries (auth, build, search, technology)
├── database/
│   ├── schema.sql                # Full SQLite schema (reference — db.go is source of truth)
│   └── seed.sql                  # Initial technologies
├── frontend/
│   ├── astro.config.mjs          # output: 'static' (NOT hybrid — removed in Astro 5.x)
│   ├── src/
│   │   ├── layouts/              # Base.astro
│   │   ├── pages/                # EN routes + es/ Spanish routes
│   │   ├── components/           # Astro components
│   │   ├── islands/              # Preact interactive (SearchIsland, InfraLive) — 2 islands only
│   │   ├── styles/               # tokens.css + global.css (CSS 2026: @layer, container queries, light-dark())
│   │   └── i18n/                 # en.json, es.json, index.ts
├── api/openapi.yaml              # API spec
├── ee/                           # Enterprise features (proprietary license)
├── scripts/                      # backup.sh, deploy.sh, deploy-frontend.sh, migrate-to-vps.sh
├── .github/
│   ├── workflows/ci.yml          # PR checks: go lint/vet/test, frontend build, Docker dry run
│   ├── workflows/deploy.yml      # Deploy: lint → Docker/GHCR → Cloudflare Pages → SSH deploy
│   ├── dependabot.yml            # Weekly updates: gomod, npm, github-actions
│   └── PULL_REQUEST_TEMPLATE.md
├── docker-compose.yml            # Production (Go + cloudflared)
├── docker-compose.dev.yml        # Dev (Go with air + Astro dev server)
├── Dockerfile                    # Multi-stage: golang:1.24-alpine → scratch
├── Dockerfile.dev                # Dev with air hot-reload
└── Makefile                      # All commands
```

## Key Commands

```bash
make dev              # Run backend + frontend in parallel
make dev-backend      # Go server with air hot-reload
make dev-frontend     # Astro dev server on :4321
make build            # Build both backend and frontend
make deploy           # Deploy both to production
make backup           # SQLite hot backup + R2 upload
make test             # Run Go tests
make lint             # Run Go vet
```

## Design System — Editorial Zen

- **Fonts:** DM Serif Display (headings) + DM Sans (body) + DM Mono (code/handles/tags)
- **Palette:** Warm off-white #F4F2EF, forest-teal accent #5C7C6E, terracotta #8A6858, olive #8A8458
- **Cards:** Specular highlight top-border, ring shadow (no CSS border), hover lift 2px
- **CSS:** @layer cascade, container queries (not media queries), light-dark() ready, fluid clamp() typography

## API Endpoints

```
GET    /health                        # Health check
GET    /api/infra                     # Server metrics (cached 30s)
GET    /api/infra/stream              # SSE real-time metrics (every 5s)

POST   /api/auth/webauthn/register/*  # Passkey registration
POST   /api/auth/webauthn/login/*     # Passkey login
GET    /api/auth/github               # GitHub OAuth redirect
GET    /api/auth/github/callback      # GitHub OAuth callback
POST   /api/auth/logout               # Destroy session
GET    /api/auth/me                   # Current builder

GET    /api/builders/:handle          # Builder profile
POST   /api/builders                  # Create builder (requires invite)

GET    /api/builds                    # List builds (?tech=&status=&sort=)
GET    /api/builds/:id                # Build detail + updates
POST   /api/builds                    # Create build (auth required)
PUT    /api/builds/:id                # Update build (owner only)

GET    /api/technologies              # List with build_count
GET    /api/technologies/:slug        # Builds using this tech

GET    /api/search?q=                 # FTS5 search with ranking

GET    /badge/:handle.svg             # SVG badge for READMEs
GET    /badge/:handle/:buildId.svg    # Build-specific badge
```

## Database

- **SQLite** with WAL mode, foreign keys ON, busy timeout 5000ms
- **Pragmas (microSD optimized):** synchronous=NORMAL, cache_size=-20000 (20MB), mmap_size=64MB, temp_store=MEMORY, page_size=8192, wal_autocheckpoint=1000
- **FTS5** virtual table `search_index` for full-text search with porter stemming
- Schema source of truth: `internal/database/db.go` (migrations run on startup)
- `database/schema.sql` is a reference copy — always keep in sync with db.go
- Backups: hourly hot backup → gzip → Cloudflare R2
- Build statuses: `building`, `launched`, `paused`, `abandoned` (NOT shipped/archived)

## i18n

- English (default): no URL prefix — `stackrigs.com/explore`
- Spanish: `/es/` prefix — `stackrigs.com/es/explore`
- Detection: Accept-Language header → `lang` cookie → builder preference
- Translations: `frontend/src/i18n/{en,es}.json`
- User content (Build Logs) is NOT translated

## CI/CD Pipeline

- **CI (ci.yml):** Runs on PRs to main. Go lint/vet/test + frontend build + Docker dry run.
- **Deploy (deploy.yml):** Runs on push to main. Lint → Docker multi-arch build → GHCR push → Trivy scan → Cloudflare Pages → SSH deploy with auto-rollback.
- **Dependabot:** Weekly updates for Go, npm, GitHub Actions. Reviewer: HTopete.
- **Branch protection:** PRs required to main. CI must pass before merge.
- **go.sum is empty locally** (no Go installed) — CI runs `go mod tidy` to generate it.
- **No package-lock.json** — CI uses `npm install` (not `npm ci`). No npm cache in setup-node.

## Important Gotchas

- `astro.config.mjs` must use `output: 'static'` — `'hybrid'` was removed in Astro 5.x
- Go error returns must be checked (golangci-lint errcheck) — use `_ =` for intentionally ignored returns like `json.Encode` to `http.ResponseWriter`
- `defer tx.Rollback()` must be wrapped: `defer func() { _ = tx.Rollback() }()`
- Dockerfile copies ALL source before `go mod tidy` (needs source to resolve dependencies)
- Config reads `WEBAUTHN_RP_ID` and `WEBAUTHN_RP_ORIGINS` (not RPID/ORIGIN)

## Philosophy — What StackRigs is NOT

- No feed, no timeline, no infinite scroll
- No likes, no reactions, no visible follower counts
- No algorithm, no engagement notifications
- No vanity metrics — freshness and completeness, not popularity
- Test for every feature: "Does it generate value if the user is alone on the platform?"

## Code Conventions

- **Go:** slog for logging, chi middleware pattern, raw SQL (no ORM), nanoid for IDs
- **CSS:** No Tailwind. CSS custom properties + @layer + container queries. No vendor prefixes unless absolutely required.
- **JS:** Minimal. Only 2 Preact islands. Everything else is static HTML from Astro.
- **HTML:** Semantic HTML5 with ARIA where needed. Native `<dialog>` for modals. Native `<details>` for expandable sections.
- **Naming:** Go files use snake_case. Frontend uses PascalCase for components, camelCase for utils.
- **Commits:** Include `Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>` when AI-assisted.

## Environment Variables

See `.env.example` for the full list. Critical ones:
- `DATABASE_PATH` — SQLite file location (default: `/data/stackrigs.db`)
- `WEBAUTHN_RP_ID` — domain for passkeys (e.g., `stackrigs.com`)
- `WEBAUTHN_RP_ORIGINS` — allowed origins (e.g., `https://stackrigs.com`)
- `GITHUB_CLIENT_ID/SECRET` — GitHub OAuth app credentials
- `TUNNEL_TOKEN` — Cloudflare Tunnel token
- `SESSION_SECRET` — 32-byte random string for session signing

## Deployment

- **Frontend:** Astro builds to static HTML → deploys to Cloudflare Pages via wrangler
- **Backend:** Docker multi-arch image (ARM64+AMD64) → GitHub Container Registry → pulled on Pi/VPS
- **CI/CD:** `.github/workflows/deploy.yml` handles both on push to main
- **Security:** Trivy vulnerability scan + SBOM generation on every Docker build
- **Rollback:** Auto-rollback on failed health checks (tags previous image, restores on failure)
- **Migration Pi → VPS:** `scripts/migrate-to-vps.sh` (rsync + tunnel update, <30 min)

## GitHub Secrets Required

| Secret | Purpose | Required |
|--------|---------|----------|
| `CLOUDFLARE_API_TOKEN` | Wrangler deploy to Pages | Yes (frontend) |
| `CLOUDFLARE_ACCOUNT_ID` | Cloudflare account | Yes (frontend) |
| `DEPLOY_HOST` | Pi/VPS IP or hostname | When backend ready |
| `DEPLOY_USER` | SSH user on server | When backend ready |
| `DEPLOY_SSH_KEY` | SSH private key | When backend ready |
| `DEPLOY_SSH_PORT` | SSH port (default: 22) | Optional |
