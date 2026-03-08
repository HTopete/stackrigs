#!/usr/bin/env bash
# Update StackRigs build #1 with current project state.
# Run AFTER deploying PR #24.
# Usage: bash scripts/update-build-1.sh
#
# Requires an active session cookie. Get it by logging in via browser
# and copying the stackrigs_session cookie value.

set -euo pipefail

API="https://api.stackrigs.com"

if [ -z "${SESSION_COOKIE:-}" ]; then
  echo "Set SESSION_COOKIE env var first:"
  echo '  export SESSION_COOKIE="your-stackrigs_session-value"'
  exit 1
fi

curl -s -X PUT "$API/api/builds/1" \
  -H "Content-Type: application/json" \
  -b "stackrigs_session=$SESSION_COOKIE" \
  -d '{
  "name": "StackRigs",
  "description": "An open index of what builders are actually building. Real stacks. Real infra. No noise. Built on a Raspberry Pi 5 in Mazatlan.",
  "status": "building",
  "repo_url": "https://github.com/HTopete/stackrigs",
  "live_url": "https://stackrigs.com",
  "what_works": "Go 1.24 + chi v5 backend con SQLite WAL mode — rapido y sin dependencias externas. Astro 5.x genera HTML estatico con solo 6 Preact islands para interactividad (auth, search, forms, infra live, uptime, explore). FTS5 full-text search con porter stemming — busqueda instantanea. GitHub OAuth + WebAuthn passkeys para auth, session cookies compartidas entre subdomains (api.stackrigs.com ↔ stackrigs.com). SSE real-time para metricas de infra con fallback automatico a polling cuando HTTP/3 rompe la conexion. Avatars optimizados: el browser hace resize a 256px y encode a WebP (q85) via Canvas API antes de subir — ~10KB por avatar, cero procesamiento server-side. Docker multi-arch (ARM64+AMD64) desde GitHub Actions → GHCR → deploy automatico al Pi. Cloudflare Tunnel expone la API sin abrir puertos. i18n completo EN/ES. Badge SVG dinamico para READMEs. Rate limiting + ETag caching en todos los endpoints.",
  "what_broke": "CSS backdrop-filter en el nav crea un containing block que rompe position:fixed en hijos — fix: usar position:absolute con calc(100dvh). Container queries de CSS no pueden auto-referenciarse — un elemento no puede ser su propio container context, tuve que usar media queries. Cloudflare apt repo no soporta Debian Trixie (Pi 5) — hay que instalar cloudflared desde .deb releases de GitHub. Pi-hole en la misma red bloquea curl a redirects de GitHub — usar wget. HTTP/3 (QUIC) de Cloudflare rompe conexiones SSE — implementamos fallback automatico a polling despues de 2 reintentos.",
  "what_id_change": "Empezaria con procesamiento de imagenes client-side desde el dia 1 — no hay encoder WebP puro en Go sin CGO, pero el Canvas API del browser lo resuelve perfecto. Separaria FRONTEND_URL de BASE_URL desde el inicio para evitar problemas con cookies cross-subdomain. Usaria un VPS desde el principio en lugar del Pi para evitar complejidad de red (dual NIC, Pi-hole, Trixie). Consideraria htmx en lugar de Preact para las interacciones mas simples.",
  "technologies": ["go", "astro", "sqlite", "cloudflare", "docker", "github-actions", "cloudflare-cdn", "react", "anthropic"]
}' | python3 -m json.tool 2>/dev/null || echo "Done (install python3 for pretty output)"

echo ""
echo "Build #1 updated successfully."
