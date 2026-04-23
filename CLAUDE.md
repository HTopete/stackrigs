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
│   │   ├── islands/              # Preact interactive — 8 islands (Auth, BuildForm, BuildUpdates, Explore, InfraLive, ProfileEdit, Search, Uptime)
│   │   ├── styles/               # tokens.css + global.css (CSS 2026: @layer, container queries, light-dark())
│   │   └── i18n/                 # en.json, es.json, index.ts
├── api/openapi.yaml              # API spec
├── ee/                           # Enterprise features (proprietary license)
├── scripts/                      # backup.sh, deploy.sh, deploy-frontend.sh, migrate-to-vps.sh, setup-pi.sh, seed-first-build.sh, update-build-1.sh
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
- **CSS:** @layer cascade, media queries for layout (container queries can't self-reference), light-dark() ready, fluid clamp() typography

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
PUT    /api/builders/me               # Update own profile (auth required)

GET    /api/builds                    # List builds (?tech=&status=&sort=&builder=)
GET    /api/builds/:id                # Build detail + updates
POST   /api/builds                    # Create build (auth required)
PUT    /api/builds/:id                # Update build (owner only)
DELETE /api/builds/:id                # Delete build + cascade (owner only)
POST   /api/builds/:id/updates       # Add milestone/update (owner only)
DELETE /api/builds/:id/updates/:uid   # Delete update (owner only)

POST   /api/upload/avatar             # Upload avatar (auth, max 512KB, WebP preferred)
GET    /uploads/*                     # Serve uploaded files (immutable cache)

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
- CSS `backdrop-filter` creates a containing block — breaks `position: fixed` on children. Use `position: absolute` instead
- CSS container queries can't self-reference — an element can't be its own container context. Use `@media` queries for layout
- Go has no pure-Go WebP encoder (no CGO in scratch Docker) — do image processing client-side with Canvas API
- Avatar upload: client resizes to 256px + encodes WebP via `canvas.toBlob('image/webp', 0.85)`, server just validates and saves
- `COOKIE_DOMAIN=.stackrigs.com` (leading dot) needed for cross-subdomain cookie sharing
- `FRONTEND_URL` is separate from `BASE_URL` — frontend redirects (post-auth) use FRONTEND_URL

## Philosophy — What StackRigs is NOT

- No feed, no timeline, no infinite scroll
- No likes, no reactions, no visible follower counts
- No algorithm, no engagement notifications
- No vanity metrics — freshness and completeness, not popularity
- Test for every feature: "Does it generate value if the user is alone on the platform?"

## Code Conventions

- **Go:** slog for logging, chi middleware pattern, raw SQL (no ORM), nanoid for IDs
- **CSS:** No Tailwind. CSS custom properties + @layer + container queries. No vendor prefixes unless absolutely required.
- **JS:** Minimal. Preact islands for interactivity. Everything else is static HTML from Astro.
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

## Master Plan (audited 2026-04-22)

Priorizado por impacto. Trabajar en orden de fase.

### Fase 1 — Seguridad y bugs críticos (antes de usuarios reales)
- [ ] **C1** — Mutex en mapas WebAuthn (race condition → panic fatal) `internal/handler/auth.go:64-65`
- [ ] **C2** — CORS permite localhost en prod si `ALLOWED_ORIGINS` no está seteado `internal/middleware/cors.go`
- [ ] **C8** — Filtros multi-tech rotos (envía `go,react` como slug literal) `frontend/src/islands/ExploreIsland.tsx:51`
- [ ] **C11** — SSE se corta a 30s por `WriteTimeout` del server `cmd/server/main.go:190`
- [ ] **M15** — FTS no se actualiza en builds nuevos (solo al arrancar) `internal/store/build.go`
- [ ] **C9** — Token GitHub guardado en texto plano `internal/database/db.go:150`
- [ ] **A1** — Rate limiter bypaseable con `X-Forwarded-For` falso `internal/middleware/ratelimit.go:120`

### Fase 2 — UX bloqueantes (signup muerto, errores, i18n)
- [ ] Arreglar loop muerto de signup (no hay flujo de registro visible) `frontend/src/pages/signin.astro`
- [ ] Reemplazar `alert()` con errores inline en signin `frontend/src/pages/signin.astro:72,101,105`
- [ ] Arreglar acentos en español `frontend/src/i18n/es.json`
- [ ] Arreglar doble mount de `AuthIsland` en Nav (2x fetch `/api/auth/me`) `frontend/src/components/Nav.astro:38,67`

### Fase 3 — Identidad del producto
- [ ] Hero: mover "Show your build, not your brand" al primer plano `frontend/src/pages/index.astro`
- [ ] Reordenar `/build/[id]`: snapshot → timeline → embed `frontend/src/pages/build/[id].astro`
- [ ] Activar dark mode (tokens ya listos, falta 1 media query) `frontend/src/styles/tokens.css`
- [ ] Popular Stacks: jerarquía visual por conteo `frontend/src/pages/index.astro:42`
- [ ] Onboarding post-login + empty states con CTAs de owner

### Fase 4 — Cover image (backend listo, frontend falta)
- [ ] `UploadCover` handler + ruta `POST /api/upload/cover` `internal/handler/upload.go`
- [ ] Tipos `coverImage` en `BuildCardData`/`BuildDetailData` + i18n `frontend/src/lib/api.ts`
- [ ] File input + WebP resize en `BuildFormIsland.tsx`
- [ ] Render en `BuildLog.astro` (hero banner) y `BuildCard.astro` (thumbnail)

### Fase 5 — Descubrimiento y formularios
- [ ] Technologies: autocomplete con chips en `BuildFormIsland.tsx:243`
- [ ] Explore: skeleton loading, clear filters, conteo de resultados `ExploreIsland.tsx`
- [ ] Arreglar páginas `/stack/[slug]` (datos mock → API real)
- [ ] Infra como filtro en Explore (hosting/db/cdn)

### Fase 6 — Performance y polish
- [ ] N+1 queries en `BuildStore.List` `internal/store/build.go:83`
- [ ] `getStaticPaths` superar límite de 100 páginas `frontend/src/lib/api.ts:214`
- [ ] `FreshnessDot`: calcular en cliente, no en build-time `frontend/src/components/FreshnessDot.astro`
- [ ] Hover cards: suavizar animación (anti-feed brand) `frontend/src/styles/global.css:272`

### Fase 7 — Infra y features finales
- [ ] Dashboard "My Builds" para el builder autenticado
- [ ] Configurar GitHub Secrets para CI auto-deploy (`DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY`)
- [ ] "Last change" changelog en landing (anti-feed, 1 línea)

---

## Audit Status (as of 2026-03-08)

### Fully Working (end-to-end)
- [x] Auth flow: GitHub OAuth + Passkeys (WebAuthn)
- [x] Profile edit: display_name, bio, website, twitter_url, avatar upload
- [x] Profile page: renders all builder links (github, twitter, website)
- [x] Build CRUD: create, read, update, delete
- [x] Build updates/milestones: create, read, delete (timeline UI)
- [x] FTS5 search
- [x] Badge SVG generation
- [x] SSE real-time infra metrics with polling fallback
- [x] Uptime history tracking
- [x] i18n EN/ES
- [x] 404 page

### Cover Image — Half-Implemented
- [x] **Backend ready:** DB column `cover_image` exists, model fields defined, store Create/Update handle it, handler accepts it
- [ ] **Frontend missing:** BuildFormIsland has no file input for cover image
- [ ] **Frontend missing:** BuildCard.astro does not render cover_image
- [ ] **Frontend missing:** BuildLog.astro does not render cover_image
- [ ] **Frontend missing:** `BuildCardData` / `BuildDetailData` types in api.ts don't include coverImage
- [ ] **Frontend missing:** `toBuildCard()` / `toBuildDetail()` transforms don't map cover_image
- [ ] **Backend missing:** No `/api/upload/cover` endpoint (need similar to avatar upload but for builds)

### Implementation Plan for Cover Image
1. Add `UploadCover` handler in `upload.go` (like avatar but saves to `uploads/covers/`, max 2MB, 1200px wide)
2. Add route `POST /api/upload/cover` in `main.go`
3. Add `coverImage` field to `BuildCardData` and `BuildDetailData` in `api.ts`
4. Add `cover_image` mapping in `toBuildCard()` and `toBuildDetail()`
5. Add file input + client-side WebP resize in `BuildFormIsland.tsx`
6. Render cover image in `BuildLog.astro` (hero banner) and `BuildCard.astro` (thumbnail)
7. Add i18n labels: `buildForm.coverImage`, `buildForm.changeCover`

### Infrastructure
- [x] Docker + Docker Compose on Pi 5
- [x] Cloudflare Tunnel exposing API
- [x] DNS configured: stackrigs.com → Tunnel (API), Pages (frontend)
- [x] Health check working: https://stackrigs.com/health
- [ ] Add GitHub secrets for CI auto-deploy: `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY`

### Remaining Tasks
- [ ] **Cover image upload** — see plan above (highest priority)
- [ ] **Dashboard "My Builds"** — no page listing current user's builds
- [ ] **Invitation codes management** — no API/UI to create/list codes
- [ ] **GitHub OAuth App setup** — needs OAuth App in GitHub Settings
- [ ] **R2 bucket for backups** — script exists, bucket not created
- [ ] **Build #1 content update** — `scripts/update-build-1.sh` ready to run after deploy
- [ ] **Playwright end-to-end audit** — test all flows in live browser
