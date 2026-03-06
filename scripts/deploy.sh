#!/usr/bin/env bash
set -euo pipefail

# StackRigs — Backend deploy with automatic rollback
# Deploys the Go backend via Docker on Pi 5 / VPS.
# Frontend is deployed separately to Cloudflare Pages (see deploy-frontend.sh).
#
# Works identically on ARM64 (Raspberry Pi 5) and AMD64 (VPS).

###############################################################################
# Config
###############################################################################
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
HEALTH_URL="http://localhost:8080/health"
HEALTH_RETRIES=10
HEALTH_INTERVAL=3
COMPOSE_FILE="$PROJECT_DIR/docker-compose.yml"
COMPOSE_PROD="$PROJECT_DIR/docker-compose.prod.yml"

# Use prod override if it exists (GHCR image instead of local build)
if [[ -f "$COMPOSE_PROD" ]]; then
  COMPOSE_CMD="docker compose -f $COMPOSE_FILE -f $COMPOSE_PROD"
else
  COMPOSE_CMD="docker compose -f $COMPOSE_FILE"
fi

###############################################################################
# Helpers
###############################################################################
log()  { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"; }
die()  { log "FATAL: $*"; exit 1; }

health_check() {
  local attempt=1
  while [[ $attempt -le $HEALTH_RETRIES ]]; do
    if curl -sf "$HEALTH_URL" > /dev/null 2>&1; then
      log "Health check passed (attempt $attempt/$HEALTH_RETRIES)"
      return 0
    fi
    log "Health check attempt $attempt/$HEALTH_RETRIES failed — retrying in ${HEALTH_INTERVAL}s"
    sleep "$HEALTH_INTERVAL"
    attempt=$((attempt + 1))
  done
  return 1
}

###############################################################################
# Pre-flight
###############################################################################
cd "$PROJECT_DIR"

command -v docker >/dev/null 2>&1 || die "docker not found"
command -v git    >/dev/null 2>&1 || die "git not found"

log "=== StackRigs Backend Deploy ==="
log "Architecture: $(uname -m)"
log "Project dir:  $PROJECT_DIR"

###############################################################################
# Step 1: Pre-deploy backup
###############################################################################
log "Step 1: Running pre-deploy backup"
if [[ -x "$SCRIPT_DIR/backup.sh" ]]; then
  "$SCRIPT_DIR/backup.sh" || log "WARNING: pre-deploy backup failed (continuing)"
else
  log "WARNING: backup.sh not found or not executable — skipping"
fi

###############################################################################
# Step 2: Save current image ID for rollback
###############################################################################
PREV_IMAGE="$($COMPOSE_CMD images -q stackrigs 2>/dev/null || true)"
log "Step 2: Previous image: ${PREV_IMAGE:-none}"

###############################################################################
# Step 3: Pull latest code
###############################################################################
log "Step 3: Pulling latest code"
git pull --ff-only || die "git pull failed — resolve conflicts manually"

###############################################################################
# Step 4: Build
###############################################################################
log "Step 4: Building Docker image"
$COMPOSE_CMD build --no-cache || die "docker compose build failed"

###############################################################################
# Step 5: Deploy
###############################################################################
log "Step 5: Starting services"
$COMPOSE_CMD up -d || die "docker compose up failed"

###############################################################################
# Step 6: Post-deploy health check
###############################################################################
log "Step 6: Waiting for health check"
if health_check; then
  log "Backend deploy successful"
  docker image prune -f > /dev/null 2>&1 || true
  exit 0
fi

###############################################################################
# Rollback
###############################################################################
log "Health check failed — initiating rollback"

if [[ -n "${PREV_IMAGE:-}" ]]; then
  log "Rolling back to image: $PREV_IMAGE"
  $COMPOSE_CMD down
  docker tag "$PREV_IMAGE" stackrigs:rollback 2>/dev/null || true
  $COMPOSE_CMD up -d
  if health_check; then
    log "Rollback successful"
  else
    die "Rollback also failed — manual intervention required"
  fi
else
  die "No previous image available for rollback — manual intervention required"
fi

exit 1
