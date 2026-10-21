# AOSphere BOS — Legal, Compliance & Terms of Service

## 1. Terms of Service & Autonomous Agent Operating Agreement
- **Human Accountability:** Autonomous AI agents operating within AOSphere act on behalf of the customer organization. The organization remains solely responsible for all commitments, purchase orders, invoices, and payroll disbursed by its authorized agents.
- **Safety Rails & Approval Thresholds:** Customer agrees not to disable or tamper with Human-in-the-Loop (HITL) approval thresholds for actions exceeding designated financial limits.
- **Acceptable Use:** The service shall not be used for illicit transactions, money laundering, market manipulation, or unverified automated inventory scraping.

---

## 2. Privacy Policy & Data Protection (GDPR / CCPA)
- **Data Ownership:** Customers retain 100% ownership and intellectual property rights over all customer records, financial ledgers, inventory data, employee records, and agent execution logs.
- **Data Minimization & Encryption:** All sensitive personal data (PII) is encrypted in transit using TLS 1.3 and at rest via AES-256 encrypted volumes. Passwords use salted cryptographic hashes.
- **Data Subject Rights:** Support for GDPR Articles 15–20 (Right of Access, Rectification, Erasure, and Data Portability via `/api/export`).
- **Zero Third-Party Telemetry Sale:** AOSphere does not monetize, sell, or share customer operational data or AI prompt context with external model providers or advertisers.

---

## 3. SOC 2 Type II Readiness Controls

| Trust Service Criteria | AOSphere Implementation |
| :--- | :--- |
| **Security (CC6.1 - CC6.3)** | Granular RBAC, cryptographic session cookies, TLS 1.3, emergency kill switch. |
| **Availability (A1.1 - A1.3)** | 99.9% uptime SLO, multi-stage health probes (`/healthz`, `/readyz`), automated hot backups. |
| **Processing Integrity (PI1.1 - PI1.5)** | Double-entry bookkeeping balance invariants, optimistic concurrency control, transactional outbox. |
| **Confidentiality (C1.1 - C1.2)** | Multi-tenant isolation (`WHERE org_id = ?`), token-hashed API keys, secret sanitization in logs. |
| **Change Management (CC8.1)** | Idempotent versioned migrations, CI/CD automated test suite, branch protection on `main`. |

---

## 4. Financial & Physical Record Reconciliation Protocol
- **Monthly Stock Audits:** Physical warehouse inventory must be counted monthly and matched against `skus.stock_quantity`. Discrepancies are adjusted via `service.AdjustStock` with type `AUDIT_ADJUSTMENT`.
- **Trial Balance Reconciliation:** Before closing monthly accounting periods, accounting officers must verify that `GET /api/finance/trial-balance` reports `is_balanced: true` with zero unallocated variance.
