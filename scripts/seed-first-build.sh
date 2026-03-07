#!/usr/bin/env bash
# =============================================================================
# StackRigs — Seed: create the first build (StackRigs itself)
# Run on the Pi after logging in via GitHub OAuth to get a session cookie.
# =============================================================================
# Usage:
#   1. Log in at https://stackrigs.com/signin (GitHub OAuth)
#   2. Copy your session cookie from browser DevTools:
#      Application > Cookies > stackrigs_session
#   3. Run: COOKIE="your_session_value" ./scripts/seed-first-build.sh
# =============================================================================

set -euo pipefail

API="${API_BASE:-https://api.stackrigs.com}"
COOKIE="${COOKIE:?Set COOKIE to your stackrigs_session value}"

echo "==> Fetching builder info..."
ME=$(curl -s -b "stackrigs_session=${COOKIE}" "${API}/api/auth/me")
BUILDER_ID=$(echo "$ME" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
HANDLE=$(echo "$ME" | grep -o '"handle":"[^"]*"' | head -1 | cut -d'"' -f4)

if [ -z "$BUILDER_ID" ] || [ "$BUILDER_ID" = "null" ]; then
  echo "ERROR: Could not get builder ID. Is your session cookie valid?"
  echo "Response: $ME"
  exit 1
fi

echo "    Builder: @${HANDLE} (ID: ${BUILDER_ID})"

echo "==> Creating build: StackRigs..."
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${API}/api/builds" \
  -b "stackrigs_session=${COOKIE}" \
  -H "Content-Type: application/json" \
  -d "{
    \"builder_id\": ${BUILDER_ID},
    \"name\": \"StackRigs\",
    \"description\": \"An open index of what builders are actually building. Real stacks, real infra, no noise.\",
    \"status\": \"building\",
    \"repo_url\": \"https://github.com/HTopete/stackrigs\",
    \"live_url\": \"https://stackrigs.com\",
    \"what_works\": \"SQLite on a Pi 5 handles 500+ req/s without breaking a sweat. Single-binary Go deployment means zero dependency management. Cloudflare Tunnel gives us HTTPS for free.\",
    \"what_broke\": \"Cloudflare apt repo does not support Debian Trixie. Pi-hole blocks curl to GitHub redirects. Dual network interfaces make ip route unpredictable.\",
    \"what_id_change\": \"Would start with Astro from day one instead of trying a SPA first. The island architecture is perfect for content-heavy sites.\",
    \"technologies\": [\"go\", \"astro\", \"sqlite\", \"cloudflare\", \"docker\", \"github-actions\"]
  }")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

if [ "$HTTP_CODE" = "201" ]; then
  BUILD_ID=$(echo "$BODY" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
  echo "    Build created! ID: ${BUILD_ID}"
  echo "    View at: https://stackrigs.com/build/${BUILD_ID}"
else
  echo "ERROR: HTTP ${HTTP_CODE}"
  echo "$BODY"
  exit 1
fi
