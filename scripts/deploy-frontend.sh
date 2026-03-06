#!/usr/bin/env bash
set -euo pipefail

# StackRigs — Frontend deploy to Cloudflare Pages
#
# Builds the Astro frontend locally (or in CI) and deploys to Cloudflare Pages
# using wrangler. Includes verification and rollback.
#
# Prerequisites:
#   - Node.js 22+ and npm
#   - wrangler CLI: npm install -g wrangler
#   - CLOUDFLARE_API_TOKEN env var or `wrangler login`
#
# Usage:
#   ./deploy-frontend.sh              # build + deploy
#   ./deploy-frontend.sh --skip-build # deploy existing dist/

###############################################################################
# Config
###############################################################################
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
FRONTEND_DIR="$PROJECT_DIR/frontend"

# Source .env for CF_PAGES_PROJECT, CF_ACCOUNT_ID, etc.
if [[ -f "$PROJECT_DIR/.env" ]]; then
  # shellcheck disable=SC1091
  set -a; source "$PROJECT_DIR/.env"; set +a
fi

PAGES_PROJECT="${CF_PAGES_PROJECT:-stackrigs}"
ACCOUNT_ID="${CF_ACCOUNT_ID:-}"
PRODUCTION_BRANCH="main"
DEPLOY_DIR="$FRONTEND_DIR/dist"
SKIP_BUILD=false
VERIFY_URL="https://stackrigs.com"

# Parse arguments
for arg in "$@"; do
  case "$arg" in
    --skip-build) SKIP_BUILD=true ;;
    *) ;;
  esac
done

###############################################################################
# Helpers
###############################################################################
log()  { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"; }
die()  { log "FATAL: $*"; exit 1; }

###############################################################################
# Pre-flight
###############################################################################
log "=== StackRigs Frontend Deploy ==="

command -v npx >/dev/null 2>&1 || die "npx not found — install Node.js 22+"

if [[ -n "$ACCOUNT_ID" ]]; then
  WRANGLER_ACCOUNT_FLAG="--account-id $ACCOUNT_ID"
else
  WRANGLER_ACCOUNT_FLAG=""
fi

###############################################################################
# Step 1: Build
###############################################################################
if [[ "$SKIP_BUILD" == "false" ]]; then
  log "Step 1: Building Astro frontend"
  cd "$FRONTEND_DIR"

  if [[ ! -d "node_modules" ]]; then
    log "Installing dependencies"
    npm ci
  fi

  npm run build || die "Frontend build failed"
  log "Build complete"
else
  log "Step 1: Skipping build (--skip-build)"
fi

if [[ ! -d "$DEPLOY_DIR" ]]; then
  die "Build output not found at $DEPLOY_DIR"
fi

###############################################################################
# Step 2: Get current deployment ID (for rollback)
###############################################################################
log "Step 2: Saving current deployment for rollback"

PREV_DEPLOYMENT_ID=""
if command -v wrangler >/dev/null 2>&1 || npx wrangler --version >/dev/null 2>&1; then
  PREV_DEPLOYMENT_ID="$(npx wrangler pages deployment list \
    --project-name "$PAGES_PROJECT" \
    $WRANGLER_ACCOUNT_FLAG 2>/dev/null \
    | grep "production" \
    | head -1 \
    | awk '{print $1}')" || true
fi
log "Previous deployment: ${PREV_DEPLOYMENT_ID:-unknown}"

###############################################################################
# Step 3: Deploy to Cloudflare Pages
###############################################################################
log "Step 3: Deploying to Cloudflare Pages"

cd "$FRONTEND_DIR"

DEPLOY_OUTPUT="$(npx wrangler pages deploy "$DEPLOY_DIR" \
  --project-name "$PAGES_PROJECT" \
  --branch "$PRODUCTION_BRANCH" \
  $WRANGLER_ACCOUNT_FLAG \
  --commit-dirty=true 2>&1)" || die "wrangler pages deploy failed: $DEPLOY_OUTPUT"

# Extract the deployment URL from wrangler output
DEPLOY_URL="$(echo "$DEPLOY_OUTPUT" | grep -oP 'https://[^\s]+\.pages\.dev' | head -1)" || true
log "Deployed: ${DEPLOY_URL:-check Cloudflare dashboard}"

###############################################################################
# Step 4: Verify deployment is live
###############################################################################
log "Step 4: Verifying deployment"

sleep 10  # Pages propagation delay

VERIFY_ATTEMPTS=5
VERIFIED=false

for i in $(seq 1 $VERIFY_ATTEMPTS); do
  HTTP_STATUS="$(curl -sf -o /dev/null -w '%{http_code}' "$VERIFY_URL" 2>/dev/null)" || HTTP_STATUS="000"
  if [[ "$HTTP_STATUS" =~ ^(200|301|302)$ ]]; then
    log "Verification passed: $VERIFY_URL returned $HTTP_STATUS (attempt $i/$VERIFY_ATTEMPTS)"
    VERIFIED=true
    break
  fi
  log "Verification attempt $i/$VERIFY_ATTEMPTS: got $HTTP_STATUS"
  sleep 5
done

if [[ "$VERIFIED" == "true" ]]; then
  log "Frontend deploy successful"
  exit 0
fi

###############################################################################
# Rollback
###############################################################################
log "Verification failed — attempting rollback"

if [[ -n "$PREV_DEPLOYMENT_ID" ]]; then
  log "Rolling back to deployment: $PREV_DEPLOYMENT_ID"
  npx wrangler pages deployment rollback "$PREV_DEPLOYMENT_ID" \
    --project-name "$PAGES_PROJECT" \
    $WRANGLER_ACCOUNT_FLAG 2>/dev/null || log "WARNING: Automatic rollback failed — use Cloudflare dashboard"
  log "Rollback initiated — check Cloudflare dashboard to confirm"
else
  log "WARNING: No previous deployment ID for rollback — use Cloudflare dashboard"
fi

exit 1
