# AO Sphere — Production Operational Runbooks & Disaster Recovery

This document provides step-by-step Standard Operating Procedures (SOPs) for operating, maintaining, backing up, and recovering AO Sphere in high-availability enterprise environments.

---

## 1. System Architecture & Core Invariants

AO Sphere is architected as an ultra-compact single-process monolith:
- **Storage Engine**: Embedded SQLite 3 in WAL (Write-Ahead Logging) mode.
- **Concurrency Model**: Single serialized writer with unlimited non-blocking concurrent readers.
- **Accounting Invariant**: Every financial transaction enforces zero-sum double-entry debit/credit balance across all sub-ledgers.
- **Audit Invariant**: SHA-256 cryptographically chained immutable audit vault.

---

## 2. Disaster Recovery & Backup Restoration

### 2.1 Backup Structure
All backup archives are taken via SQLite's atomic online `VACUUM INTO` command after issuing a `PRAGMA wal_checkpoint(TRUNCATE)`. Snapshots are stored either:
- **Plain SQLite**: `aosphere_backup_YYYYMMDD_HHMMSS.db`
- **Encrypted**: `aosphere_backup_YYYYMMDD_HHMMSS.db.enc` (AES-256-GCM authenticated envelope, SHA-256 derived key, 12-byte nonce, 16-byte authentication tag).

### 2.2 Point-in-Time Recovery Procedure

#### Step A: Access Recovery Controls
You can restore snapshots via:
1. **Web UI**: Navigate to **Governance & Audit -> Backups & Disaster Recovery -> Click "Restore"** on target snapshot.
2. **REST API**:
   ```bash
   curl -X POST http://localhost:8080/api/backup/restore \
     -H "Content-Type: application/json" \
     -d '{"filename": "aosphere_prod_20261104_120000.db.enc", "passphrase": "<YOUR_PASSPHRASE>"}'
   ```
3. **Emergency Manual CLI Restoration**:
   If the container or server is halted:
   ```bash
   # 1. Stop container
   docker compose stop aosphere

   # 2. Decrypt backup (if encrypted)
   # Run via the aosphere CLI or python decryption helper
   python -c "
   import hashlib, os
   from cryptography.hazmat.primitives.ciphers.aead import AESGCM
   key = hashlib.sha256(b'<YOUR_PASSPHRASE>').digest()
   aesgcm = AESGCM(key)
   with open('data/backups/snapshot.db.enc', 'rb') as f:
       data = f.read()
   nonce, ct = data[:12], data[12:]
   pt = aesgcm.decrypt(nonce, ct, None)
   with open('data/aosphere.db', 'wb') as f:
       f.write(pt)
   "

   # 3. Clean up stale WAL and shared memory files
   rm -f data/aosphere.db-wal data/aosphere.db-shm

   # 4. Verify database integrity
   sqlite3 data/aosphere.db "PRAGMA integrity_check;"

   # 5. Restart service
   docker compose start aosphere
   ```

### 2.3 Restoring from Cloudflare R2 / AWS S3
If the local server disk is lost:
```bash
# 1. Download snapshot from S3 bucket
aws s3 cp s3://aosphere-production-backups/backups/aosphere_prod_latest.db.enc ./data/backups/

# 2. Invoke Restore API or follow Step 2.2 above.
```

---

## 3. SQLite WAL Maintenance & Optimization

### 3.1 Manual WAL Checkpointing
Under high write loads, the `-wal` file can expand. AO Sphere automatically checkpoints on every snapshot and clean shutdown. To manually force a checkpoint:
```bash
# Via SQLite CLI
sqlite3 /var/lib/aosphere/data/aosphere.db "PRAGMA wal_checkpoint(TRUNCATE);"
```

### 3.2 Database Compaction (VACUUM)
To reclaim unused pages after large batch deletions:
```bash
sqlite3 /var/lib/aosphere/data/aosphere.db "VACUUM;"
```

---

## 4. Zero-Downtime Rolling Upgrades

Always execute upgrades using the automated runbook script:
```bash
./deploy/scripts/upgrade.sh
```

### What `upgrade.sh` Executes:
1. Validates healthz on existing running instance.
2. Triggers an atomic encrypted backup snapshot (`POST /api/backup/create`).
3. Verifies PRAGMA integrity_check on the new snapshot.
4. Pulls/builds the new container image.
5. Performs rolling restart with automatic transactional migrations (`internal/db/migration.go`).
6. Polls `/readyz` for 60 seconds.
7. **Automated Rollback**: If `/readyz` fails, automatically restores the pre-upgrade snapshot and restarts the previous version.

---

## 5. Kubernetes High Availability & Storage Failover

Because SQLite enforces single-writer file locking:
- **Always use `StatefulSet`** (never multi-replica Deployment).
- **Persistent Volume**: Use `ReadWriteOnce` with NVMe / SSD block storage (`storageClassName: standard`).
- **Warm Standby**: To configure active-passive warm standby across regions, configure the backup sidecar to sync snapshots to Cloudflare R2 every 15 minutes. In the event of primary cluster loss, launch the secondary Helm chart pointing to the restored volume.

---

## 6. Incident Triage Matrix

| Alert / Symptom | Severity | Probable Root Cause | Resolution Action |
|---|---|---|---|
| `GeneralLedgerImbalance` | Critical | Debit/Credit mismatch | Run `/api/reports/balance-sheet` and inspect unposted journal entries in `journal_entries` table. |
| `OutboxDeadLettersDetected` | Warning | External webhook endpoint down | Inspect `/api/webhooks/deliveries` and click "Redeliver" after third-party fixes their endpoint. |
| `StaleOrMissingBackups` | Critical | Backup cron failing or disk full | Check `df -h /var/lib/aosphere/data` and verify S3 credentials via `/api/backup/config`. |
| `DatabaseIntegrityFailed` | Page | Filesystem or hardware corruption | Halt container, restore latest verified backup snapshot from S3, restart. |
| `HighRequestLatency` | Warning | Long-running report or lock wait | Ensure all analytical queries run with `_pragma=query_only(1)` read connections. |

