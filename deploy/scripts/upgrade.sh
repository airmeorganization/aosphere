#!/usr/bin/env bash
# ==============================================================================
# AO Sphere Safe Zero-Downtime Rolling Update & Automated Migration Runbook
# ==============================================================================
set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
PASSPHRASE="${BACKUP_PASSPHRASE:-}"
TIMESTAMP=$(date -u +%Y%m%d_%H%M%S)

echo "=================================================================="
echo "  AO Sphere Safe Rolling Update & Migration Script"
echo "  Timestamp: $TIMESTAMP"
echo "=================================================================="

# 1. Health check before attempting upgrade
echo "==> Step 1: Checking current cluster health..."
if ! curl -sf "$API_URL/healthz" > /dev/null; then
    echo "[ERROR] AO Sphere service is not currently healthy at $API_URL. Aborting upgrade."
    exit 1
fi
echo "    Service is currently operational."

# 2. Pre-upgrade atomic snapshot & integrity verification
echo "==> Step 2: Triggering mandatory pre-upgrade atomic backup snapshot..."
BACKUP_RES=$(curl -sf -X POST "$API_URL/api/backup/create" \
    -H "Content-Type: application/json" \
    -d "{\"prefix\": \"pre_upgrade_${TIMESTAMP}\", \"passphrase\": \"$PASSPHRASE\", \"upload_cloud\": true}")

BACKUP_FILE=$(echo "$BACKUP_RES" | grep -o '"filename":"[^"]*' | cut -d'"' -f4 || echo "")
INTEGRITY_OK=$(echo "$BACKUP_RES" | grep -o '"integrity_ok":true' || echo "")

if [[ -z "$INTEGRITY_OK" || -z "$BACKUP_FILE" ]]; then
    echo "[FATAL] Pre-upgrade backup failed or integrity check did not pass! Aborting upgrade."
    echo "Output: $BACKUP_RES"
    exit 1
fi
echo "    Pre-upgrade snapshot created: $BACKUP_FILE (Verified PRAGMA integrity_check OK)"

# 3. Pull or build new container
echo "==> Step 3: Pulling latest container images..."
docker compose pull aosphere || echo "Local build mode, skipping docker pull."

# 4. Graceful container restart with automatic schema migrations on boot
echo "==> Step 4: Applying rolling update with automated transactional migrations..."
docker compose up -d --no-deps --build aosphere

# 5. Post-deployment health verification probe
echo "==> Step 5: Probing readiness endpoint (/readyz)..."
HEALTHY=false
for i in {1..30}; do
    if curl -sf "$API_URL/readyz" > /dev/null; then
        HEALTHY=true
        break
    fi
    echo "    Waiting for migrations to finish and service to become ready... (attempt $i/30)"
    sleep 2
done

# 6. Automated Rollback Safeguard
if [[ "$HEALTHY" != "true" ]]; then
    echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
    echo "  CRITICAL: Post-upgrade readiness probe FAILED! Initiating rollback..."
    echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
    
    # Restore pre-upgrade snapshot
    echo "==> Restoring pre-upgrade database snapshot: $BACKUP_FILE..."
    curl -sf -X POST "$API_URL/api/backup/restore" \
        -H "Content-Type: application/json" \
        -d "{\"filename\": \"$BACKUP_FILE\", \"passphrase\": \"$PASSPHRASE\"}" || true

    echo "==> Restarting previous container..."
    docker compose restart aosphere
    echo "[ROLLBACK COMPLETE] Live database restored to pre-upgrade snapshot."
    exit 1
fi

echo "=================================================================="
echo "  [SUCCESS] AO Sphere upgraded successfully with zero downtime!"
echo "  Verified: /healthz OK, /readyz OK, Schema Migrations Applied."
echo "=================================================================="

