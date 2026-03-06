<p align="center">
  <strong>SR</strong>
</p>

<h1 align="center">StackRigs</h1>

<p align="center">
  An open index of what builders are actually building.<br>
  Real stacks. Real infra. No noise.
</p>

<p align="center">
  <a href="https://stackrigs.com">Website</a> &middot;
  <a href="https://stackrigs.com/infra">Live Infra</a> &middot;
  <a href="https://stackrigs.com/about">About</a>
</p>

---

## What is StackRigs?

StackRigs is a structured directory where builders document what they're building, with what stack, and what's actually working (or breaking). It's not a social network. There's no feed, no likes, no followers, no algorithm.

Every build has a **Build Log** with specific fields:
- **Stack** &mdash; the actual technologies used
- **Infrastructure** &mdash; hosting, database, CDN, CI/CD, monitoring
- **What Works** &mdash; what's going well
- **What Broke** &mdash; what failed
- **What I'd Change** &mdash; retrospective decisions

Browse by stack. Search by technology. See how real builders use real tools in production.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.24, chi router, SQLite (WAL + FTS5) |
| Frontend | Astro 5.18, Preact islands (~14KB JS total) |
| Auth | Passkeys (WebAuthn) + GitHub OAuth |
| Infra | Raspberry Pi 5 (16GB) in Mazatlan, Mexico |
| CDN | Cloudflare Pages + Tunnel |
| Search | SQLite FTS5 with porter stemming |
| Real-time | Server-Sent Events for /infra metrics |
| i18n | English + Spanish (no URL prefixes for default locale) |

**Yes, every page you see on stackrigs.com is served from a Raspberry Pi in a closet in Mazatlan.**

## Self-Hosting

```bash
git clone https://github.com/htopete/stackrigs.git
cd stackrigs
cp .env.example .env
# Edit .env with your values
docker compose up -d
```

The app will be available at `http://localhost:8080`.

See `.env.example` for all configuration options.

### Requirements

- Docker and Docker Compose
- Or: Go 1.24+ and Node.js 22+ (for development without Docker)

## Development

```bash
# Run backend + frontend in parallel
make dev

# Backend only (Go with hot-reload)
make dev-backend

# Frontend only (Astro dev server on :4321)
make dev-frontend

# Run tests
make test

# Build for production
make build
```

## Project Structure

```
stackrigs/
├── cmd/server/          # Go entry point
├── internal/            # Go backend (handlers, stores, middleware)
├── frontend/            # Astro 5.18 frontend
│   ├── src/pages/       # Routes
│   ├── src/components/  # Astro components (0 JS)
│   ├── src/islands/     # Preact islands (Search + InfraLive)
│   └── src/styles/      # CSS 2026 (@layer, container queries, light-dark())
├── database/            # Schema + seed SQL
├── ee/                  # Enterprise features (separate license)
├── api/                 # OpenAPI 3.1 spec
├── scripts/             # Backup, deploy, migrate
└── .github/workflows/   # CI/CD
```

## API

Full API documentation: [`api/openapi.yaml`](api/openapi.yaml)

Badges for your README:

```markdown
![StackRigs](https://stackrigs.com/badge/your-handle.svg)
```

## Philosophy

- **No feed.** The homepage is an index: search, recent builds, popular stacks.
- **No likes.** A build is measured by completeness and freshness, not popularity.
- **No followers.** You can bookmark builders privately. No public counts.
- **No algorithm.** Explicit navigation: by stack, tool, search.
- **No engagement notifications.** Only a monthly self-reminder to update.

Test for every new feature: *"Does it generate value if the user is alone on the platform?"*
If not, it's a social feature in disguise.

## Contributing

Contributions are welcome. Please read the license section below before contributing.

Areas where contributions are especially valuable:
- Translations (i18n)
- New technologies in `database/seed.sql`
- Bug fixes
- Accessibility improvements

## License

The core of StackRigs is licensed under the [Functional Source License, Version 1.1, Apache 2.0 Future License (FSL-1.1-ALv2)](LICENSE).

This means:
- You can read, use, modify, and self-host the code
- You cannot use it to create a competing commercial service
- After 2 years, each version automatically converts to Apache 2.0

The `ee/` directory contains Enterprise Edition features under a [separate proprietary license](ee/LICENSE).

---

<p align="center">
  Built with Go + Astro + SQLite. Running on a Raspberry Pi 5 in Mazatlan, Mexico.<br>
  <strong>Show your build, not your brand.</strong>
</p>
