#!/usr/bin/env bash
set -euo pipefail

# StackRigs — Migrate from Raspberry Pi 5 to VPS
#
# This script runs FROM the Pi (or current host) and pushes everything
# to the target VPS. It handles both:
#   - Backend: Docker containers (Go API + cloudflared)
#   - Frontend: Cloudflare Pages (update tunnel to point to new VPS)
#
# Usage:
#   ./migrate-to-vps.sh user@vps-ip [/opt/stackrigs]

###############################################################################
# Config
###############################################################################
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

VPS_HOST="${1:-}"
VPS_PATH="${2:-/opt/stackrigs}"

if [[ -z "$VPS_HOST" ]]; then
  cat <<'USAGE'
StackRigs — Migration Tool (2026 Architecture)

Usage:
  ./migrate-to-vps.sh user@vps-ip [remote-path]

Arguments:
  user@vps-ip    SSH destination (e.g. root@203.0.113.10)
  remote-path    Installation path on VPS (default: /opt/stackrigs)

Architecture:
  Frontend: Cloudflare Pages (Astro) — no migration needed
  Backend:  Docker on Pi/VPS (Go API + cloudflared tunnel)

Prerequisites on the VPS:
  - Docker & Docker Compose v2 installed
  - SSH access configured (key-based recommended)
  - At least 1 GB free disk space

What this script does:
  1. Creates a fresh SQLite backup on the current host
  2. Syncs project files to the VPS via rsync
  3. Copies the database (including WAL files) and backups
  4. Copies .env and reminds you to update TUNNEL_TOKEN
  5. Builds and starts Docker services on the VPS
  6. Runs post-migration health check
  7. Reminds you to update Cloudflare Tunnel to point to new VPS

USAGE
  exit 1
fi

# Source .env for config values
if [[ -f "$PROJECT_DIR/.env" ]]; then
  # shellcheck disable=SC1091
  set -a; source "$PROJECT_DIR/.env"; set +a
fi

###############################################################################
# Helpers
###############################################################################
log()  { echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"; }
die()  { log "FATAL: $*"; exit 1; }

confirm() {
  read -rp "$1 [y/N] " response
  [[ "$response" =~ ^[Yy]$ ]] || die "Aborted by user"
}

###############################################################################
# Pre-flight checks
###############################################################################
command -v rsync >/dev/null 2>&1 || die "rsync is required"
command -v ssh   >/dev/null 2>&1 || die "ssh is required"

log "=== StackRigs Migration ==="
log "  Source:      $PROJECT_DIR"
log "  Destination: $VPS_HOST:$VPS_PATH"
log ""
log "  Backend:  Will be migrated to $VPS_HOST"
log "  Frontend: Stays on Cloudflare Pages (no action needed)"
log ""
confirm "Proceed with migration?"

###############################################################################
# Step 1: Backup current database
###############################################################################
log "Step 1/7: Creating pre-migration backup"
if [[ -x "$SCRIPT_DIR/backup.sh" ]]; then
  "$SCRIPT_DIR/backup.sh"
else
  log "WARNING: backup.sh not found — skipping pre-migration backup"
fi

###############################################################################
# Step 2: Prepare remote directory
###############################################################################
log "Step 2/7: Preparing remote directory"
ssh "$VPS_HOST" "mkdir -p $VPS_PATH/{data,backups,scripts}"

###############################################################################
# Step 3: Sync project files (excludes runtime data)
###############################################################################
log "Step 3/7: Syncing project files"
rsync -avz --progress \
  --exclude '.git' \
  --exclude 'data/' \
  --exclude 'backups/' \
  --exclude '.env' \
  --exclude 'tmp/' \
  --exclude 'frontend/node_modules/' \
  --exclude 'frontend/dist/' \
  --exclude 'frontend/.astro/' \
  "$PROJECT_DIR/" "$VPS_HOST:$VPS_PATH/"

###############################################################################
# Step 4: Sync data (database + WAL files) and backups
###############################################################################
log "Step 4/7: Syncing database and backups"

if [[ -d "$PROJECT_DIR/data" ]]; then
  # Important: sync all SQLite files together (.db, .db-wal, .db-shm)
  rsync -avz --progress "$PROJECT_DIR/data/" "$VPS_HOST:$VPS_PATH/data/"
  log "Database synced (including WAL/SHM files if present)"
fi

if [[ -d "$PROJECT_DIR/backups" ]]; then
  rsync -avz --progress "$PROJECT_DIR/backups/" "$VPS_HOST:$VPS_PATH/backups/"
fi

###############################################################################
# Step 5: Configure .env on VPS
###############################################################################
log "Step 5/7: Environment configuration"

if [[ -f "$PROJECT_DIR/.env" ]]; then
  log "Copying .env to VPS (you MUST update TUNNEL_TOKEN afterward)"
  scp "$PROJECT_DIR/.env" "$VPS_HOST:$VPS_PATH/.env"
else
  log "No .env found locally — copying .env.example"
  scp "$PROJECT_DIR/.env.example" "$VPS_HOST:$VPS_PATH/.env"
fi

cat <<'REMINDER'

  *** IMPORTANT — TUNNEL MIGRATION ***

  The Cloudflare Tunnel must be updated to point to the new VPS:

    Option A: Create a new tunnel for the VPS
      1. cloudflared tunnel create stackrigs-vps
      2. Update TUNNEL_TOKEN in .env on the VPS
      3. In Cloudflare Zero Trust dashboard, configure:
           - api.stackrigs.com -> http://stackrigs:8080

    Option B: Reuse the existing tunnel
      1. Copy the same TUNNEL_TOKEN (already done)
      2. Stop cloudflared on the Pi
      3. Start cloudflared on the VPS

  The Cloudflare Pages frontend does NOT need changes —
  it talks to the backend through the tunnel regardless
  of where the tunnel terminates.

REMINDER

###############################################################################
# Step 6: Build and start services on VPS
###############################################################################
log "Step 6/7: Building and starting services on VPS"
confirm "Build and start Docker services on $VPS_HOST now?"

ssh "$VPS_HOST" "cd $VPS_PATH && docker compose build && docker compose up -d"

log "Waiting 15s for services to start..."
sleep 15

###############################################################################
# Step 7: Post-migration health check
###############################################################################
log "Step 7/7: Post-migration health check"

HEALTH_OK=false
for i in 1 2 3 4 5; do
  if ssh "$VPS_HOST" "curl -sf http://localhost:8080/health > /dev/null 2>&1"; then
    log "Health check PASSED (attempt $i)"
    HEALTH_OK=true
    break
  fi
  log "Health check attempt $i failed — retrying in 5s"
  sleep 5
done

if [[ "$HEALTH_OK" == "false" ]]; then
  log "WARNING: Health check failed — check logs on VPS:"
  log "  ssh $VPS_HOST 'cd $VPS_PATH && docker compose logs'"
fi

###############################################################################
# Summary
###############################################################################
cat <<EOF

=== Migration Complete ===

Backend ($VPS_HOST):
  1. SSH in and verify:       ssh $VPS_HOST
  2. Check logs:              cd $VPS_PATH && docker compose logs -f
  3. Update TUNNEL_TOKEN:     nano $VPS_PATH/.env
  4. Restart tunnel:          cd $VPS_PATH && docker compose restart cloudflared
  5. Verify backend health:   curl http://localhost:8080/health

Frontend (Cloudflare Pages):
  - No migration needed — frontend is deployed to Cloudflare Pages
  - It connects to the backend through the Cloudflare Tunnel
  - Once the tunnel points to the VPS, everything works automatically

Cleanup (after confirming everything works):
  1. Stop services on Pi:     cd $PROJECT_DIR && docker compose down
  2. Delete old tunnel:       cloudflared tunnel delete stackrigs-pi
  3. Set up cron on VPS:      crontab < $VPS_PATH/crontab.example

EOF
