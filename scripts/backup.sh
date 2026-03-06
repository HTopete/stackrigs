#!/usr/bin/env bash
set -euo pipefail

# StackRigs — SQLite Hot Backup with R2 Upload
# Safe to run while the database is in use (uses sqlite3 .backup).
# Designed for cron: exits 0 on success, non-zero on failure.
#
# Features:
#   - Hot backup via sqlite3 .backup (safe with WAL mode)
#   - Integrity check on the backup copy
#   - Compress with gzip
#   - Upload to Cloudflare R2 (if credentials configured)
#   - Local + remote retention cleanup

###############################################################################
# Config
###############################################################################
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Source .env if present
if [[ -f "$PROJECT_DIR/.env" ]]; then
  # shellcheck disable=SC1091
  set -a; source "$PROJECT_DIR/.env"; set +a
fi

DB_PATH="${DATABASE_PATH:-/data/stackrigs.db}"
BACKUP_DIR="${BACKUP_DIR:-$PROJECT_DIR/backups}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
TIMESTAMP="$(date -u +%Y-%m-%d-%H%M%S)"
BACKUP_NAME="stackrigs-${TIMESTAMP}.db"
BACKUP_GZ="${BACKUP_NAME}.gz"
LOG_FILE="${BACKUP_DIR}/backup.log"

# R2 config (optional — skip upload if not set)
R2_BUCKET="${BACKUP_R2_BUCKET:-}"
R2_ENDPOINT="${BACKUP_R2_ENDPOINT:-}"
R2_ACCESS_KEY="${BACKUP_R2_ACCESS_KEY:-}"
R2_SECRET_KEY="${BACKUP_R2_SECRET_KEY:-}"

###############################################################################
# Helpers
###############################################################################
log() {
  local msg="[$(date '+%Y-%m-%d %H:%M:%S')] $*"
  echo "$msg" | tee -a "$LOG_FILE"
}

die() {
  log "ERROR: $*"
  exit 1
}

###############################################################################
# Pre-checks
###############################################################################
mkdir -p "$BACKUP_DIR"

# If running outside Docker, the DB might be at a host-relative path
if [[ ! -f "$DB_PATH" ]]; then
  HOST_DB="$PROJECT_DIR/data/stackrigs.db"
  if [[ -f "$HOST_DB" ]]; then
    DB_PATH="$HOST_DB"
  else
    die "Database not found at $DB_PATH or $HOST_DB"
  fi
fi

command -v sqlite3 >/dev/null 2>&1 || die "sqlite3 is not installed"

###############################################################################
# Step 1: Hot backup
###############################################################################
log "Starting backup: $BACKUP_NAME"

BACKUP_PATH="${BACKUP_DIR}/${BACKUP_NAME}"

if sqlite3 "$DB_PATH" ".backup '${BACKUP_PATH}'"; then
  SIZE="$(du -h "$BACKUP_PATH" | cut -f1)"
  log "Backup complete: $BACKUP_NAME ($SIZE)"
else
  die "sqlite3 .backup failed for $DB_PATH"
fi

###############################################################################
# Step 2: Integrity check on the backup
###############################################################################
log "Running integrity check on backup"
INTEGRITY="$(sqlite3 "$BACKUP_PATH" "PRAGMA integrity_check;" 2>&1)"
if [[ "$INTEGRITY" != "ok" ]]; then
  rm -f "$BACKUP_PATH"
  die "Integrity check failed: $INTEGRITY"
fi
log "Integrity check passed"

###############################################################################
# Step 3: Compress
###############################################################################
log "Compressing backup"
gzip -f "$BACKUP_PATH"
BACKUP_PATH="${BACKUP_DIR}/${BACKUP_GZ}"
GZ_SIZE="$(du -h "$BACKUP_PATH" | cut -f1)"
log "Compressed: $BACKUP_GZ ($GZ_SIZE)"

###############################################################################
# Step 4: Upload to Cloudflare R2 (if configured)
###############################################################################
upload_to_r2() {
  if [[ -z "$R2_BUCKET" || -z "$R2_ENDPOINT" || -z "$R2_ACCESS_KEY" || -z "$R2_SECRET_KEY" ]]; then
    log "R2 not configured — skipping remote upload"
    return 0
  fi

  log "Uploading to R2: s3://${R2_BUCKET}/backups/${BACKUP_GZ}"

  # Check for rclone first (preferred), fall back to aws cli
  if command -v rclone >/dev/null 2>&1; then
    # Configure rclone on the fly
    export RCLONE_CONFIG_R2_TYPE=s3
    export RCLONE_CONFIG_R2_PROVIDER=Cloudflare
    export RCLONE_CONFIG_R2_ACCESS_KEY_ID="$R2_ACCESS_KEY"
    export RCLONE_CONFIG_R2_SECRET_ACCESS_KEY="$R2_SECRET_KEY"
    export RCLONE_CONFIG_R2_ENDPOINT="$R2_ENDPOINT"
    export RCLONE_CONFIG_R2_NO_CHECK_BUCKET=true

    if rclone copyto "$BACKUP_PATH" "r2:${R2_BUCKET}/backups/${BACKUP_GZ}" --quiet; then
      log "R2 upload complete (rclone)"
    else
      log "WARNING: R2 upload failed (rclone)"
      return 1
    fi
  elif command -v aws >/dev/null 2>&1; then
    export AWS_ACCESS_KEY_ID="$R2_ACCESS_KEY"
    export AWS_SECRET_ACCESS_KEY="$R2_SECRET_KEY"

    if aws s3 cp "$BACKUP_PATH" "s3://${R2_BUCKET}/backups/${BACKUP_GZ}" \
        --endpoint-url "$R2_ENDPOINT" --quiet; then
      log "R2 upload complete (aws cli)"
    else
      log "WARNING: R2 upload failed (aws cli)"
      return 1
    fi
  else
    log "WARNING: Neither rclone nor aws cli found — skipping R2 upload"
    return 0
  fi
}

upload_to_r2

###############################################################################
# Step 5: Local retention — delete backups older than RETENTION_DAYS
###############################################################################
log "Cleaning local backups older than ${RETENTION_DAYS} days"

DELETED=0
while IFS= read -r old_backup; do
  rm -f "$old_backup"
  log "Deleted old backup: $(basename "$old_backup")"
  DELETED=$((DELETED + 1))
done < <(find "$BACKUP_DIR" -maxdepth 1 \( -name "stackrigs-*.db" -o -name "stackrigs-*.db.gz" \) -type f -mtime +"$RETENTION_DAYS" 2>/dev/null)

log "Retention cleanup done: $DELETED file(s) removed"

###############################################################################
# Step 6: Remote retention (R2) — delete objects older than RETENTION_DAYS
###############################################################################
cleanup_r2() {
  if [[ -z "$R2_BUCKET" || -z "$R2_ENDPOINT" || -z "$R2_ACCESS_KEY" || -z "$R2_SECRET_KEY" ]]; then
    return 0
  fi

  if ! command -v rclone >/dev/null 2>&1; then
    log "rclone not available — skipping R2 retention cleanup"
    return 0
  fi

  log "Cleaning R2 backups older than ${RETENTION_DAYS} days"

  export RCLONE_CONFIG_R2_TYPE=s3
  export RCLONE_CONFIG_R2_PROVIDER=Cloudflare
  export RCLONE_CONFIG_R2_ACCESS_KEY_ID="$R2_ACCESS_KEY"
  export RCLONE_CONFIG_R2_SECRET_ACCESS_KEY="$R2_SECRET_KEY"
  export RCLONE_CONFIG_R2_ENDPOINT="$R2_ENDPOINT"
  export RCLONE_CONFIG_R2_NO_CHECK_BUCKET=true

  rclone delete "r2:${R2_BUCKET}/backups/" \
    --min-age "${RETENTION_DAYS}d" \
    --quiet 2>/dev/null || log "WARNING: R2 retention cleanup failed"

  log "R2 retention cleanup done"
}

cleanup_r2

log "Backup pipeline finished successfully"
exit 0
