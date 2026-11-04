#!/usr/bin/env bash
# ==============================================================================
# AO Sphere Hetzner / Linux VPS Production Provisioning Script
# ==============================================================================
set -euo pipefail

echo "==> [AO Sphere] Starting automated production setup..."

# 1. Root verification
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root (or with sudo)." 
   exit 1
fi

# 2. Update and install prerequisites
echo "==> Installing system dependencies..."
apt-get update -qq
apt-get install -y -qq curl git ufw fail2ban jq ca-certificates

# 3. Docker Installation (if not present)
if ! command -v docker &> /dev/null; then
    echo "==> Installing Docker CE and Compose plugin..."
    curl -fsSL https://get.docker.com | sh
fi

# 4. Security Firewall configuration
echo "==> Configuring UFW firewall rules..."
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 443/udp
ufw --force enable

# 5. Persistent directories setup
APP_DIR="/opt/aosphere"
DATA_DIR="/var/lib/aosphere/data"
BACKUP_DIR="/var/lib/aosphere/backups"

echo "==> Preparing persistent data directories..."
mkdir -p "$APP_DIR" "$DATA_DIR" "$BACKUP_DIR"
chown -R 10001:10001 /var/lib/aosphere
chmod 750 "$DATA_DIR"

# 6. Copy or Clone Repository
if [[ ! -f "$APP_DIR/docker-compose.yml" ]]; then
    echo "==> Deploying application configs to $APP_DIR..."
    if [[ -f "docker-compose.yml" ]]; then
        cp -r . "$APP_DIR/"
    else
        git clone https://github.com/aosphere/aosphere.git "$APP_DIR"
    fi
fi

cd "$APP_DIR"

# 7. Generate .env if not exists
if [[ ! -f ".env" ]]; then
    echo "==> Generating production .env with secure random tokens..."
    cp .env.example .env
    RAND_SESSION=$(openssl rand -hex 32)
    RAND_PASSPHRASE=$(openssl rand -hex 24)
    sed -i "s/replace_with_a_64_character_hex_random_secret_key_here/$RAND_SESSION/" .env
    sed -i "s/replace_with_secure_long_backup_encryption_passphrase/$RAND_PASSPHRASE/" .env
fi

# 8. Start production stack
echo "==> Bootstrapping Docker Compose services..."
docker compose up -d --build

# 9. Health verification
echo "==> Waiting for service to report healthy..."
for i in {1..30}; do
    if curl -s -f http://localhost:8080/healthz > /dev/null; then
        echo "==> [SUCCESS] AO Sphere is live and reporting healthy!"
        echo "    Dashboard:  http://localhost:8080"
        echo "    Data Dir:   $DATA_DIR"
        echo "    Backups:    $BACKUP_DIR"
        exit 0
    fi
    sleep 2
done

echo "==> [WARNING] Health check timed out. Inspect logs with: docker compose logs"
exit 1

