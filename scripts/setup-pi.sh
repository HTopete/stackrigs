#!/usr/bin/env bash
set -euo pipefail

# StackRigs — Raspberry Pi 5 Initial Setup (2026)
#
# Run ON the Pi 5 after a fresh Raspberry Pi OS 64-bit (Bookworm) install.
#
# Usage:
#   scp setup-pi.sh user@pi-ip:~/
#   ssh user@pi-ip
#   sudo bash setup-pi.sh
#
# Order of operations:
#   1. System update + essentials
#   2. Install Docker CE + Compose v2 (official Debian arm64 repo)
#   3. Install cloudflared (Cloudflare apt repo for arm64)
#   4. Create Cloudflare Tunnel + configure DNS (MUST be done before services start)
#   5. Create /opt/stackrigs deploy structure + .env
#   6. Firewall — SSH only (tunnel handles all inbound)
#   7. Authenticate GHCR + pull image + start services
#   8. Verify health + install cron jobs

###############################################################################
# Config
###############################################################################
DEPLOY_DIR="/opt/stackrigs"
GHCR_IMAGE="ghcr.io/htopete/stackrigs:latest"
GITHUB_USER="HTopete"
TUNNEL_NAME="stackrigs"
DOMAIN="stackrigs.com"
API_SERVICE="http://stackrigs:8080"

###############################################################################
# Helpers
###############################################################################
log()     { echo -e "\n[setup] $*"; }
die()     { echo -e "\n[setup] FATAL: $*"; exit 1; }
confirm() { read -rp "[setup] $1 [Y/n] " r; [[ "${r:-Y}" =~ ^[Yy]$ ]]; }

###############################################################################
# Pre-flight
###############################################################################
if [[ $EUID -ne 0 ]]; then
  die "Run as root: sudo bash setup-pi.sh"
fi

ARCH="$(uname -m)"
log "StackRigs Pi 5 Setup (2026)"
log "  Arch:       $ARCH"
log "  OS:         $(. /etc/os-release && echo "$PRETTY_NAME")"
log "  Deploy dir: $DEPLOY_DIR"

[[ "$ARCH" == "aarch64" ]] || log "WARNING: Expected aarch64, got $ARCH"

###############################################################################
# Step 1: System update + essentials
###############################################################################
log "Step 1/8: System update"
apt-get update -y && apt-get upgrade -y
apt-get install -y \
  curl wget git sqlite3 jq ufw \
  ca-certificates gnupg lsb-release

###############################################################################
# Step 2: Docker CE + Compose v2 (official Debian repo for arm64)
# Ref: https://docs.docker.com/engine/install/debian/
###############################################################################
log "Step 2/8: Docker"

if command -v docker &>/dev/null; then
  log "Already installed: $(docker --version)"
else
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/debian/gpg \
    | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg

  CODENAME="$(. /etc/os-release && echo "$VERSION_CODENAME")"
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/debian $CODENAME stable" \
    > /etc/apt/sources.list.d/docker.list

  apt-get update -y
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

  systemctl enable --now docker
  log "Installed: $(docker --version) + $(docker compose version)"
fi

REAL_USER="${SUDO_USER:-$(whoami)}"
if [[ "$REAL_USER" != "root" ]]; then
  usermod -aG docker "$REAL_USER"
  log "Added $REAL_USER to docker group (re-login to take effect)"
fi

###############################################################################
# Step 3: cloudflared (Cloudflare apt repo for arm64)
# Ref: https://pkg.cloudflare.com/
###############################################################################
log "Step 3/8: cloudflared"

if command -v cloudflared &>/dev/null; then
  log "Already installed: $(cloudflared --version)"
else
  mkdir -p /usr/share/keyrings
  curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg \
    -o /usr/share/keyrings/cloudflare-main.gpg

  echo "deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] \
https://pkg.cloudflare.com/cloudflared $(. /etc/os-release && echo "$VERSION_CODENAME") main" \
    > /etc/apt/sources.list.d/cloudflared.list

  apt-get update -y
  apt-get install -y cloudflared
  log "Installed: $(cloudflared --version)"
fi

###############################################################################
# Step 4: Cloudflare Tunnel — create + configure DNS
# This MUST happen before docker compose up (cloudflared needs TUNNEL_TOKEN)
#
# We use a remotely-managed tunnel (2026 best practice):
#   - Config lives on Cloudflare, not local files
#   - Manage routes from Zero Trust dashboard or API
#   - Single TUNNEL_TOKEN is all the container needs
###############################################################################
log "Step 4/8: Cloudflare Tunnel"

cat <<'TUNNEL_GUIDE'

  *** CLOUDFLARE TUNNEL SETUP ***

  Go to the Cloudflare Zero Trust dashboard:
    https://one.dash.cloudflare.com

  1. Networks > Tunnels > Create a tunnel
  2. Connector type: Cloudflared
  3. Name: stackrigs
  4. Skip the install step (Docker handles it)
  5. Add public hostname:
       Subdomain: (leave blank for apex)
       Domain:    stackrigs.com
       Path:      api/*
       Service:   HTTP://stackrigs:8080
  6. Copy the TUNNEL_TOKEN from the install command
     (it's the long string after --token)

TUNNEL_GUIDE

TUNNEL_TOKEN=""
while [[ -z "$TUNNEL_TOKEN" ]]; do
  read -rp "[setup] Paste your TUNNEL_TOKEN: " TUNNEL_TOKEN
done

log "Tunnel token captured (${#TUNNEL_TOKEN} chars)"

###############################################################################
# Step 5: Deploy directory + .env
###############################################################################
log "Step 5/8: Deploy directory"

mkdir -p "$DEPLOY_DIR"/{data,backups,scripts}

# Clone repo to get compose + scripts
if [[ ! -f "$DEPLOY_DIR/docker-compose.yml" ]]; then
  log "Cloning StackRigs repo..."
  git clone --depth 1 https://github.com/HTopete/stackrigs.git /tmp/stackrigs-clone
  cp /tmp/stackrigs-clone/docker-compose.yml "$DEPLOY_DIR/"
  cp /tmp/stackrigs-clone/.env.example "$DEPLOY_DIR/.env"
  cp /tmp/stackrigs-clone/crontab.example "$DEPLOY_DIR/"
  cp /tmp/stackrigs-clone/scripts/*.sh "$DEPLOY_DIR/scripts/"
  chmod +x "$DEPLOY_DIR/scripts/"*.sh
  rm -rf /tmp/stackrigs-clone
fi

# Production override: use GHCR image instead of local build
cat > "$DEPLOY_DIR/docker-compose.prod.yml" <<PROD
services:
  stackrigs:
    image: ${GHCR_IMAGE}
PROD

# Generate SESSION_SECRET
SESSION_SECRET="$(openssl rand -hex 32)"

# Write .env with real values
sed -i \
  -e "s|TUNNEL_TOKEN=.*|TUNNEL_TOKEN=${TUNNEL_TOKEN}|" \
  -e "s|SESSION_SECRET=.*|SESSION_SECRET=${SESSION_SECRET}|" \
  "$DEPLOY_DIR/.env"

log "SESSION_SECRET generated"
log "TUNNEL_TOKEN written to .env"

# Optional: GitHub OAuth
cat <<'OAUTH'

  *** GITHUB OAUTH (optional — can do later) ***

  Create an OAuth App at:
    https://github.com/settings/developers

    Homepage URL:      https://stackrigs.com
    Authorization URL: https://stackrigs.com/api/auth/github/callback

OAUTH

if confirm "Configure GitHub OAuth now?"; then
  read -rp "  GITHUB_CLIENT_ID: " GH_CLIENT_ID
  read -rsp "  GITHUB_CLIENT_SECRET: " GH_CLIENT_SECRET
  echo ""
  sed -i \
    -e "s|GITHUB_CLIENT_ID=.*|GITHUB_CLIENT_ID=${GH_CLIENT_ID}|" \
    -e "s|GITHUB_CLIENT_SECRET=.*|GITHUB_CLIENT_SECRET=${GH_CLIENT_SECRET}|" \
    "$DEPLOY_DIR/.env"
  log "GitHub OAuth configured"
else
  log "Skipping — configure later in $DEPLOY_DIR/.env"
fi

###############################################################################
# Step 6: Firewall
###############################################################################
log "Step 6/8: Firewall"

# Detect Pi-hole — if running, allow DNS + admin web interface
PIHOLE_DETECTED=false
if command -v pihole &>/dev/null || systemctl is-active --quiet pihole-FTL 2>/dev/null; then
  PIHOLE_DETECTED=true
  log "Pi-hole detected — will keep DNS (53) and admin (80) open for LAN"
fi

ufw --force reset > /dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow ssh

if [[ "$PIHOLE_DETECTED" == "true" ]]; then
  # Allow DNS from local network only
  LAN_SUBNET="$(ip -4 route show default | awk '{print $3}' | sed 's/\.[0-9]*$/.0\/24/')"
  ufw allow from "$LAN_SUBNET" to any port 53
  ufw allow from "$LAN_SUBNET" to any port 80
  log "Allowed DNS (53) and Pi-hole admin (80) from $LAN_SUBNET"
fi

ufw --force enable

log "Firewall configured. Port 8080 NOT exposed (tunnel handles inbound)"

###############################################################################
# Step 7: GHCR auth + pull + start
###############################################################################
log "Step 7/8: Start services"

log "Authenticating with GitHub Container Registry..."
log "Create a PAT at: https://github.com/settings/tokens/new"
log "  Scope needed: read:packages"
echo ""

read -rsp "[setup] GitHub PAT (read:packages): " GITHUB_PAT
echo ""
echo "$GITHUB_PAT" | docker login ghcr.io -u "$GITHUB_USER" --password-stdin
log "GHCR authenticated"

cd "$DEPLOY_DIR"

log "Pulling $GHCR_IMAGE"
docker pull "$GHCR_IMAGE"

log "Starting stackrigs + cloudflared..."
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Health check
log "Waiting for health check..."
HEALTHY=false
for i in $(seq 1 15); do
  if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
    log "Health check PASSED (attempt $i/15)"
    HEALTHY=true
    break
  fi
  echo "  attempt $i/15 — retrying in 3s..."
  sleep 3
done

if [[ "$HEALTHY" == "true" ]]; then
  log "StackRigs is running"
  docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
else
  log "WARNING: Health check failed. Debug with:"
  log "  cd $DEPLOY_DIR && docker compose -f docker-compose.yml -f docker-compose.prod.yml logs"
fi

###############################################################################
# Step 8: Cron jobs
###############################################################################
log "Step 8/8: Cron jobs"

if confirm "Install cron jobs (hourly backups, health monitor, maintenance)?"; then
  crontab "$DEPLOY_DIR/crontab.example"
  log "Cron installed"
fi

# Shell aliases for convenience
BASHRC="/home/${REAL_USER}/.bashrc"
if [[ -f "$BASHRC" ]] && ! grep -q "stackrigs aliases" "$BASHRC"; then
  cat >> "$BASHRC" <<ALIASES

# stackrigs aliases
alias sr='docker compose -f $DEPLOY_DIR/docker-compose.yml -f $DEPLOY_DIR/docker-compose.prod.yml'
alias srl='docker compose -f $DEPLOY_DIR/docker-compose.yml -f $DEPLOY_DIR/docker-compose.prod.yml logs -f'
ALIASES
  log "Added sr/srl aliases to $BASHRC"
fi

###############################################################################
# Summary
###############################################################################
PI_IP="$(hostname -I | awk '{print $1}')"

cat <<SUMMARY

============================================================
  StackRigs Pi 5 — Setup Complete
============================================================

  $DEPLOY_DIR/
    docker-compose.yml       (base)
    docker-compose.prod.yml  (GHCR image override)
    .env                     (secrets)
    data/stackrigs.db        (SQLite — created on first run)
    backups/                 (hourly hot backups)

  Commands:
    cd $DEPLOY_DIR
    docker compose -f docker-compose.yml -f docker-compose.prod.yml logs -f
    docker compose -f docker-compose.yml -f docker-compose.prod.yml ps
    docker compose -f docker-compose.yml -f docker-compose.prod.yml restart

  An alias was added to ~/.bashrc:
    sr  → docker compose (with both files)
    srl → docker compose logs -f

  GitHub Secrets (for CI/CD auto-deploy):
    DEPLOY_HOST     $PI_IP
    DEPLOY_USER     $REAL_USER
    DEPLOY_SSH_KEY  (paste your SSH private key)

  Verify tunnel:
    curl -sf https://stackrigs.com/health && echo "OK"

============================================================

SUMMARY
