# StackRigs

An open index of what builders are actually building. Real stacks. Real infra. No noise.

**URL:** https://stackrigs.com
**Author:** Ruben Topete (@htopete_dev)

## Architecture

```
[Browser] → Cloudflare Edge
                ├── Cloudflare Pages (Astro 5.18 static + Server Islands)
                └── Cloudflare Tunnel → Pi 5 / VPS (Go 1.24 API + SQLite)
```

- **Backend:** Go 1.24 + chi v5 + modernc.org/sqlite (pure Go, no CGO)
- **Frontend:** Astro 5.18 (hybrid output) + Preact islands
- **Database:** SQLite with WAL mode + FTS5 for search
- **Auth:** Passkeys (WebAuthn) + GitHub OAuth, session cookies (no JWT)
- **Infra:** Raspberry Pi 5 (16GB) in Mazatlán, Docker Compose, Cloudflare Tunnel
- **CDN:** Cloudflare Pages for static, Cloudflare free tier for API caching

## Project Structure

```
stackrigs/
├── cmd/server/main.go           # Go entry point
├── internal/
│   ├── config/                   # Env vars config
│   ├── database/                 # SQLite connection + migrations + FTS5
│   ├── handler/                  # HTTP handlers (auth, builds, badges, SSE, infra)
│   ├── middleware/               # CORS, rate limit, ETag, auth, logging
│   ├── model/                    # Go structs
│   └── store/                    # Database queries
├── database/
│   ├── schema.sql                # Full SQLite schema (source of truth)
│   └── seed.sql                  # Initial technologies
├── frontend/
│   ├── astro.config.mjs
│   ├── src/
│   │   ├── layouts/              # Base.astro, Page.astro
│   │   ├── pages/                # Routes (index, explore, search, infra, about, [handle], build/[id], stack/[slug])
│   │   ├── components/           # Astro components (BuildCard, StackTag, StatusBadge, etc.)
│   │   ├── islands/              # Preact interactive (SearchIsland, InfraLive) — ONLY 2 islands, ~14KB total JS
│   │   ├── styles/               # tokens.css + global.css (CSS 2026: @layer, container queries, light-dark())
│   │   └── i18n/                 # en.json, es.json, index.ts
├── api/openapi.yaml              # API spec
├── scripts/                      # backup.sh, deploy.sh, deploy-frontend.sh, migrate-to-vps.sh
├── .github/workflows/deploy.yml  # CI/CD: Pages + GHCR + SSH deploy
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
- **FTS5** virtual table `search_index` for full-text search with porter stemming
- Schema source of truth: `database/schema.sql`
- Migrations run automatically on startup in `internal/database/db.go`
- Backups: hourly hot backup → gzip → Cloudflare R2

## i18n

- English (default): no URL prefix — `stackrigs.com/builds`
- Spanish: `/es/` prefix — `stackrigs.com/es/builds`
- Detection: Accept-Language header → `lang` cookie → builder preference
- Translations: `frontend/src/i18n/{en,es}.json`
- User content (Build Logs) is NOT translated — displayed in the language the builder wrote it

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

## Environment Variables

See `.env.example` for the full list. Critical ones:
- `DATABASE_PATH` — SQLite file location (default: `/data/stackrigs.db`)
- `WEBAUTHN_RPID` — domain for passkeys (e.g., `stackrigs.com`)
- `GITHUB_CLIENT_ID/SECRET` — GitHub OAuth app credentials
- `TUNNEL_TOKEN` — Cloudflare Tunnel token
- `SESSION_SECRET` — 32-byte random string for session signing

## Deployment

- **Frontend:** Astro builds to static HTML → deploys to Cloudflare Pages via wrangler
- **Backend:** Docker image → GitHub Container Registry → pulled on Pi/VPS
- **CI/CD:** `.github/workflows/deploy.yml` handles both on push to main
- **Migration Pi → VPS:** `scripts/migrate-to-vps.sh` (rsync + tunnel update, <30 min)
