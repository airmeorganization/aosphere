#!/usr/bin/env bash
# ==============================================================================
# AO Sphere Automated Scheduled Backup & Cloud Sync Script
# ==============================================================================
set -euo pipefail

API_URL="${API_URL:-http://localhost:8080}"
PASSPHRASE="${BACKUP_PASSPHRASE:-}"
RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
BACKUP_DIR="${BACKUP_DIR:-/var/lib/aosphere/data/backups}"

echo "[$(date -u)] Starting automated backup routine..."

# 1. Trigger online snapshot
BACKUP_RES=$(curl -sf -X POST "$API_URL/api/backup/create" \
    -H "Content-Type: application/json" \
    -d "{\"prefix\": \"scheduled\", \"passphrase\": \"$PASSPHRASE\", \"upload_cloud\": true}")

echo "[$(date -u)] Backup response: $BACKUP_RES"

# 2. Local retention pruning (prune local files older than RETENTION_DAYS)
if [[ -d "$BACKUP_DIR" ]]; then
    echo "[$(date -u)] Pruning local snapshots older than $RETENTION_DAYS days..."
    find "$BACKUP_DIR" -type f \( -name "*.db" -o -name "*.enc" \) -mtime "+$RETENTION_DAYS" -exec rm -f {} + || true
fi

echo "[$(date -u)] Scheduled backup routine completed."

