# AOSphere BOS — Disaster Recovery Playbook

## Target Service Level Objectives
- **Recovery Time Objective (RTO):** $< 1$ Hour
- **Recovery Point Objective (RPO):** $< 15$ Minutes

---

## 1. Automated Backup Architecture
AOSphere features built-in hot backup capabilities using SQLite's online backup API via `internal/backup/backup.go`:
- **Backup Location:** `/var/lib/aosphere/backups/` (mounted to off-site cloud storage volume, e.g. AWS S3 / Cloud Storage / Cloudflare R2).
- **Snapshot Frequency:** Cron-triggered snapshot every 15 minutes.
- **WAL Checkpointing:** Active transactions are safely checkpointed with `PRAGMA wal_checkpoint(PASSIVE)`.

---

## 2. Disaster Recovery Scenarios & Procedures

### Scenario A: Accidental Data Corruption or Catastrophic Host Failure
1. **Provision New Host / Container Instance:**
   ```bash
   docker-compose down
   ```
2. **Download Most Recent Verified Backup:**
   ```bash
   # Identify newest snapshot:
   aws s3 cp s3://aosphere-backups/prod/aosphere_backup_latest.db /var/lib/aosphere/data/aosphere.db
   ```
3. **Verify Integrity of Restored Database:**
   ```bash
   sqlite3 /var/lib/aosphere/data/aosphere.db "PRAGMA integrity_check;"
   # Expected output: "ok"
   ```
4. **Run Invariant Checks (Ledger Balance):**
   ```bash
   sqlite3 /var/lib/aosphere/data/aosphere.db "
     SELECT SUM(CASE WHEN entry_type='debit' THEN amount_cents ELSE 0 END) AS total_debits,
            SUM(CASE WHEN entry_type='credit' THEN amount_cents ELSE 0 END) AS total_credits 
     FROM journal_lines;"
   # Confirm total_debits == total_credits
   ```
5. **Start AOSphere Service:**
   ```bash
   docker-compose up -d
   ```
6. **Verify Health Endpoint:**
   ```bash
   curl -f http://localhost:8080/readyz
   ```

---

### Scenario B: WAL File Lock Contention / Zombie Lock Recovery
If a hard power cut leaves a dangling `-wal` or `-shm` lock:
1. Stop all AOSphere processes:
   ```bash
   kill -TERM $(pgrep aosphere) || docker-compose stop aosphere
   ```
2. Force recovery using SQLite CLI:
   ```bash
   sqlite3 /var/lib/aosphere/data/aosphere.db "PRAGMA wal_checkpoint(TRUNCATE);"
   ```
3. If WAL recovery completes cleanly, restart the daemon.

---

### Scenario C: Outbox Dead-Letter Replay
When external webhooks or background handlers encounter extended network outages:
1. Inspect dead-letter events:
   ```sql
   SELECT id, event_type, retry_count, created_at FROM outbox_events WHERE status = 'dead_letter';
   ```
2. Once the external service has recovered, requeue dead-letter events:
   ```sql
   UPDATE outbox_events SET status = 'pending', retry_count = 0 WHERE status = 'dead_letter';
   ```
3. The background outbox worker will automatically pick up and dispatch the events.

---

## 3. Zero-Downtime Database Migration Policy
1. **Append-Only Schema Evolution:** Never drop or rename columns in an active release. All new migrations must add nullable columns or new tables with defaults.
2. **Backward Compatibility Guarantee:** Code version $N+1$ must operate seamlessly on schema $N$, and code version $N$ must operate on schema $N+1$.
3. **Phased Rollout:** Apply schema migrations on container boot (`db.RunMigrations(db)` executes idempotently inside a single serialized transaction), followed by rolling container restart.
