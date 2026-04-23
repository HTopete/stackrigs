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
│   ├── handler/                  # HTTP handlers (auth, builds, badges, SSE, infra, health, upload)
│   ├── middleware/               # CORS, rate limit, ETag, auth, logging
│   ├── model/                    # Go structs
│   └── store/                    # Database queries (auth, build, search, technology, uptime)
├── database/
│   ├── schema.sql                # Full SQLite schema (reference — db.go is source of truth)
│   └── seed.sql                  # Initial technologies
├── frontend/
│   ├── astro.config.mjs          # output: 'static' (NOT hybrid — removed in Astro 5.x)
│   ├── src/
│   │   ├── layouts/              # Base.astro, Page.astro
│   │   ├── pages/                # EN routes + es/ Spanish routes
│   │   │   ├── index.astro       # Landing — hero, recent builds, popular stacks
│   │   │   ├── explore.astro     # Filtered build browser
│   │   │   ├── my-builds.astro   # Authenticated user's build dashboard
│   │   │   ├── new-build.astro   # Create build form
│   │   │   ├── signin.astro      # Auth page (GitHub OAuth + Passkeys)
│   │   │   ├── [handle].astro    # Builder profile
│   │   │   ├── build/[id].astro  # Build detail (log + timeline + embed)
│   │   │   ├── build/[id]/edit.astro
│   │   │   └── stack/[slug].astro  # Technology page (real API data)
│   │   ├── components/           # Astro components (BuildCard, BuildLog, Nav, FreshnessDot…)
│   │   ├── islands/              # Preact interactive islands:
│   │   │   ├── AuthIsland.tsx    # Nav auth state + dropdown
│   │   │   ├── BuildFormIsland.tsx  # Create/edit build (chips autocomplete + cover upload)
│   │   │   ├── BuildUpdatesIsland.tsx
│   │   │   ├── ExploreIsland.tsx  # Filters, skeleton loading, clear filters, result count
│   │   │   ├── InfraLive.tsx
│   │   │   ├── ProfileEditIsland.tsx
│   │   │   ├── SearchIsland.tsx
│   │   │   └── UptimeBar.tsx
│   │   ├── styles/               # tokens.css + global.css (CSS 2026: @layer, container queries, light-dark())
│   │   └── i18n/                 # en.json, es.json, index.ts
├── api/openapi.yaml              # API spec
├── ee/                           # Enterprise features (proprietary license)
├── scripts/                      # backup.sh, deploy.sh, deploy-frontend.sh, migrate-to-vps.sh, setup-pi.sh
├── .github/
│   ├── workflows/ci.yml          # PR checks: go lint/vet/test, frontend build, Docker dry run
│   ├── workflows/deploy.yml      # Deploy: lint → Docker ARM64/GHCR → Cloudflare Pages (backend deploy is manual)
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
- **Cards:** Specular highlight top-border, ring shadow (no CSS border), hover shows border ring + title underline (no bounce)
- **CSS:** @layer cascade, media queries for layout, `light-dark()` active, fluid `clamp()` typography
- **Dark mode:** `color-scheme: light dark` in `tokens.css` — responds to OS preference automatically

## API Endpoints

```
GET    /health                          # Health check

GET    /api/infra                       # Server metrics (cached 30s)
GET    /api/infra/stream                # SSE real-time metrics (every 5s) — WriteTimeout cleared

POST   /api/auth/webauthn/register/*    # Passkey registration
POST   /api/auth/webauthn/login/*       # Passkey login
GET    /api/auth/github                 # GitHub OAuth redirect
GET    /api/auth/github/callback        # GitHub OAuth callback → redirects new users to /new-build?welcome=1
POST   /api/auth/logout                 # Destroy session
GET    /api/auth/me                     # Current builder

GET    /api/builders/:handle            # Builder profile
POST   /api/builders                    # Create builder (requires invite)
PUT    /api/builders/me                 # Update own profile (auth required)

GET    /api/builds                      # List builds (?tech=go&tech=react&status=&sort=&builder=)
GET    /api/builds/:id                  # Build detail + updates
POST   /api/builds                      # Create build (auth required)
PUT    /api/builds/:id                  # Update build (owner only)
DELETE /api/builds/:id                  # Delete build + cascade (owner only)
POST   /api/builds/:id/updates          # Add milestone/update (owner only)
DELETE /api/builds/:id/updates/:uid     # Delete update (owner only)

POST   /api/upload/avatar               # Upload avatar (auth, max 512KB, WebP preferred)
POST   /api/upload/cover/:buildId       # Upload build cover image (auth, owner only, max 2MB)
GET    /uploads/*                       # Serve uploaded files (immutable cache)

GET    /api/technologies                # List with build_count and category
GET    /api/technologies/:slug          # Tech detail + builds using it

GET    /api/search?q=                   # FTS5 search with ranking (updates in real-time on create/update/delete)

GET    /badge/:handle.svg               # SVG badge for READMEs
GET    /badge/:handle/:buildId.svg      # Build-specific badge
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
- Translations: `frontend/src/i18n/{en.json, es.json}` — all keys must be in sync
- User content (Build Logs) is NOT translated

## CI/CD Pipeline

- **CI (ci.yml):** Runs on PRs to main. Go lint/vet/test + frontend build + Docker dry run.
- **Deploy (deploy.yml):** Runs on push to main. Builds ARM64 Docker image → pushes to GHCR → deploys frontend to Cloudflare Pages. **Backend deploy is manual** (Pi is on private LAN).
- **Backend deploy:** SSH into Pi, then `docker pull ghcr.io/htopete/stackrigs:latest && docker compose up -d`
- **Dependabot:** Weekly updates for Go, npm, GitHub Actions. Reviewer: HTopete.
- **Branch protection:** PRs required to main. CI must pass before merge.
- **go.sum is empty locally** (no Go installed) — CI runs `go mod tidy` to generate it.
- **No package-lock.json committed** — CI uses `npm install` (not `npm ci`). No npm cache in setup-node.

## Important Gotchas

- `astro.config.mjs` must use `output: 'static'` — `'hybrid'` was removed in Astro 5.x
- Go error returns must be checked (golangci-lint errcheck) — use `_ =` for intentionally ignored returns like `json.Encode` to `http.ResponseWriter`
- `defer tx.Rollback()` must be wrapped: `defer func() { _ = tx.Rollback() }()`
- Dockerfile copies ALL source before `go mod tidy` (needs source to resolve dependencies)
- Config reads `WEBAUTHN_RP_ID` and `WEBAUTHN_RP_ORIGINS` (not RPID/ORIGIN)
- CSS `backdrop-filter` creates a containing block — breaks `position: fixed` on children. Use `position: absolute` instead
- CSS container queries can't self-reference — an element can't be its own container context. Use `@media` queries for layout
- Go has no pure-Go WebP encoder (no CGO in scratch Docker) — do image processing client-side with Canvas API
- Avatar upload: client resizes to 256px + encodes WebP via `canvas.toBlob('image/webp', 0.85)`, server validates and saves
- Cover upload: client resizes to 1200px wide + encodes WebP, uploaded to `POST /api/upload/cover/:buildId` after build save
- `COOKIE_DOMAIN=.stackrigs.com` (leading dot) needed for cross-subdomain cookie sharing
- `FRONTEND_URL` is separate from `BASE_URL` — frontend redirects (post-auth) use FRONTEND_URL
- **WebAuthn session maps** use `sync.RWMutex` + 5-min TTL goroutine — never access `regSessions`/`loginSessions` without the mutex
- **FTS5 index** is updated in real-time in `BuildHandler.Create/Update/Delete` — `RebuildIndex()` only runs on startup
- **Multi-tech filter** uses repeated `?tech=go&tech=react` params — NOT comma-joined. Backend reads `q["tech"]` (slice)
- **`buildStore.List` N+1 eliminated** — technologies loaded in one batch IN query, not per-build
- **Rate limiter** uses `CF-Connecting-IP` header (Cloudflare Tunnel sets this, clients can't spoof it)
- **SSE WriteTimeout** — `http.NewResponseController(w).SetWriteDeadline(time.Time{})` is called at start of `InfraStream` to clear the 30s server timeout
- **GitHub OAuth token** — intentionally NOT persisted to DB. `access_token` column in `github_connections` always stores `''`
- **CORS prod safety** — `filterProdOrigins()` strips localhost origins when `ENV=prod|production`, even if `ALLOWED_ORIGINS` contains them

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
- **Commits:** Include `Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>` when AI-assisted.

## Environment Variables

See `.env.example` for the full list. Critical ones:
- `DATABASE_PATH` — SQLite file location (default: `/data/stackrigs.db`)
- `WEBAUTHN_RP_ID` — domain for passkeys (e.g., `stackrigs.com`)
- `WEBAUTHN_RP_ORIGINS` — allowed origins (e.g., `https://stackrigs.com`)
- `GITHUB_CLIENT_ID/SECRET` — GitHub OAuth app credentials
- `TUNNEL_TOKEN` — Cloudflare Tunnel token
- `SESSION_SECRET` — 32-byte random string for session signing
- `ENV` — set to `prod` or `production` in production (enables CORS prod filter)

## Deployment

- **Frontend:** Astro builds to static HTML → deploys to Cloudflare Pages via wrangler (automatic on merge to main)
- **Backend:** Docker ARM64 image → GitHub Container Registry → SSH into Pi and pull manually
- **Backend deploy command:**
  ```bash
  docker pull ghcr.io/htopete/stackrigs:latest
  docker compose up -d
  ```
- **Security:** Trivy vulnerability scan + SBOM generation on every Docker build
- **Migration Pi → VPS:** `scripts/migrate-to-vps.sh` (rsync + tunnel update, <30 min)

## GitHub Secrets Required

| Secret | Purpose | Required |
|--------|---------|----------|
| `CLOUDFLARE_API_TOKEN` | Wrangler deploy to Pages | Yes (frontend) |
| `CLOUDFLARE_ACCOUNT_ID` | Cloudflare account | Yes (frontend) |
| `DEPLOY_HOST` | Pi/VPS IP or hostname | For auto SSH deploy |
| `DEPLOY_USER` | SSH user on server | For auto SSH deploy |
| `DEPLOY_SSH_KEY` | SSH private key | For auto SSH deploy |
| `DEPLOY_SSH_PORT` | SSH port (default: 22) | Optional |

## Audit Status (as of 2026-04-22)

### Fully Working (end-to-end)
- [x] Auth flow: GitHub OAuth + Passkeys (WebAuthn) — with mutex protection
- [x] Profile edit: display_name, bio, website, twitter_url, avatar upload
- [x] Profile page: builder links, empty state CTA for owner
- [x] Build CRUD: create, read, update, delete
- [x] Build updates/milestones: create, read, delete (timeline UI)
- [x] Cover image upload: Canvas WebP resize → `POST /api/upload/cover/:id` → renders as hero + card thumbnail
- [x] FTS5 search — updates in real-time (no restart needed)
- [x] Badge SVG generation
- [x] SSE real-time infra metrics with polling fallback (no 30s disconnect)
- [x] Uptime history tracking
- [x] i18n EN/ES — all accents correct
- [x] 404 page
- [x] Dark mode (OS preference)
- [x] My Builds dashboard (`/my-builds`) — list, edit, delete
- [x] Explore: skeleton loading, clear filters, result count, tech grouped by category
- [x] Stack pages (`/stack/[slug]`) — real API data, all slugs generated statically
- [x] Tech autocomplete with chips in build form
- [x] Onboarding: new GitHub OAuth users → `/new-build?welcome=1`

### Next Priorities
- [ ] **Invitation codes management** — no API/UI to create/list invite codes
- [ ] **R2 bucket for backups** — `scripts/backup.sh` exists but bucket not created
- [ ] **GitHub SSH secrets** — add `DEPLOY_HOST`, `DEPLOY_USER`, `DEPLOY_SSH_KEY` for auto-deploy
- [ ] **Playwright end-to-end audit** — test all flows in live browser after deploy
- [ ] **Infrastructure fields on builds** — hosting/cdn/cicd/monitoring columns don't exist in DB yet (BuildLog.astro has the UI, store doesn't)
- [ ] **Build #1 content update** — `scripts/update-build-1.sh` ready to run

### Known Limitations (not blocking)
- `getStaticPaths` for builder handles de-dupes from builds list — builders without any builds won't get a static page (404 on direct visit until SSG rebuild)
- Badge SVG text width is approximate for ASCII — CJK/emoji in build names will misalign
- `SessionSecret` env var is defined but never used (sessions are random tokens in DB, not HMAC-signed cookies)
- No tests — CI runs go vet + golangci-lint but no `*_test.go` files exist yet
