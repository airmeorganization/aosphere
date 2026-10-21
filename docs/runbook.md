# AOSphere BOS — Operational Runbook & Incident Response

## 1. System Architecture & Components
- **Core Binary:** Single Go monolith (`/app/aosphere`).
- **Database:** Embedded SQLite in WAL mode with connection serialization (`MaxOpenConns(1)` for writes).
- **Network Interfaces:**
  - `8080/tcp`: Internal HTTP, SSE, and Prometheus endpoints.
  - `443/tcp`: Ingress via Caddy / Nginx reverse proxy.
- **Diagnostics:**
  - `GET /healthz`: Process liveness probe.
  - `GET /readyz`: Database and migration readiness probe.
  - `GET /metrics`: Prometheus telemetry exposition.

---

## 2. Common Alerts & Remediation Procedures

### Alert: `HighHTTPErrorRate` (5xx > 5%)
1. **Check Application Logs:**
   ```bash
   docker logs --tail=100 aosphere | grep '"level":"ERROR"'
   ```
2. **Inspect Trace / Request IDs:**
   Filter logs by `request_id` from failing requests.
3. **Check Database Health:**
   ```bash
   curl http://localhost:8080/readyz
   ```

### Alert: `GeneralLedgerImbalance`
1. **Query Ledger Imbalance:**
   ```sql
   SELECT SUM(CASE WHEN entry_type='debit' THEN amount_cents ELSE 0 END) -
          SUM(CASE WHEN entry_type='credit' THEN amount_cents ELSE 0 END) AS imbalance_cents
   FROM journal_lines;
   ```
2. **Identify Mismatched Journal Entries:**
   ```sql
   SELECT journal_entry_id, 
          SUM(CASE WHEN entry_type='debit' THEN amount_cents ELSE 0 END) AS debits,
          SUM(CASE WHEN entry_type='credit' THEN amount_cents ELSE 0 END) AS credits
   FROM journal_lines
   GROUP BY journal_entry_id
   HAVING debits != credits;
   ```
3. **Remediation:** If an unbalanced entry is discovered, post an adjusting entry using `service.PostJournalEntry` with explicit audit reasoning.

### Alert: `OutboxDeadLettersDetected`
1. **Inspect Dead-Letter Queue:**
   ```bash
   curl -H "Authorization: Bearer <ADMIN_KEY>" http://localhost:8080/api/diagnostics/outbox
   ```
2. **Check Downstream Connectivity:** Verify if webhook endpoints or external APIs are reachable.
3. **Re-drive Events:** Once downstream service is restored, reset status to `pending`.

---

## 3. Incident Postmortem Template
When a P1/P2 incident occurs:
1. **Summary:** Brief description of the failure and user impact.
2. **Timeline:** Exact timestamps (UTC) of detection, response, mitigation, and resolution.
3. **Root Cause:** 5-Whys analysis identifying the underlying technical or operational trigger.
4. **Action Items:** P0/P1 corrective tasks with owners and due dates to prevent recurrence.
