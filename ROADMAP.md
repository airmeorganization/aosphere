# AOSphere BOS — Production-Readiness Master Roadmap

To turn AOSphere from a working BOS prototype into a real production-level application, you need to do more than fix bugs or add features. You need to make it secure, financially reliable, recoverable, testable, scalable and trustworthy enough to operate a real business with AI agents.

The recommended path is to keep the existing **Go + SQLite + JavaScript** architecture for the initial hardening phase. Don't rebuild everything or introduce microservices before the current business workflows are reliable.

The roadmap below is organised into implementation phases, with concrete actions to turn into development sprints.

---

## Production-Readiness Roadmap Sequence

> **Recommended sequence**: Complete the gates in order rather than building every feature simultaneously.

| Phase | Title | Focus Area |
| :--- | :--- | :--- |
| **01** | **Stabilise the codebase** | Canonical runtime, architecture documentation, reproducible builds, config system. |
| **02** | **Secure the application** | Authentication, authorization (RBAC), sessions, secrets, network boundaries. |
| **03** | **Protect business operations** | Approvals, validation, transactions, idempotency, and auditability. |
| **04** | **Make data trustworthy** | Double-entry ledger, migrations, integrity constraints, backups, and recovery. |
| **05** | **Build a reliable agent platform** | MCP permissions, tool contracts, execution tracking, and guardrails. |
| **06** | **Finish product workflows** | Complete UI states, error handling, external integrations, and user management. |
| **07** | **Test and observe everything** | Automated testing, CI/CD, structured logs, metrics, alerts, and security checks. |
| **08** | **Deploy and operate** | Production infrastructure, incident response, restore drills, and release gates. |
| **09** | **Launch readiness** | Threat modeling, penetration testing, compliance, SLA/SLO definition. |

---

## Phase 1 — Stabilise the Codebase

**Goal:** Establish one predictable application that every developer can build, test, and run.

### Implementation Checklist (10/10)
- [x] **Choose Go as canonical runtime**: Formally adopt Go as the single production runtime and deprecate or re-purpose `aosphere_server.py`.
- [x] **Resolve runtime discrepancies**: Synchronize any features present in Python into Go, eliminating divergence.
- [x] **Pin Go toolchain & dependencies**: Lock supported Go version (e.g. Go 1.22+ / 1.23) and ensure `go.mod` / `go.sum` reproducibility.
- [x] **Define deployment targets & setup**: Document Windows, Linux, and macOS local development and production targets.
- [x] **Document architecture**: Environment variables, database locations, API routes, and MCP transports (Stdio & SSE).
- [x] **Unified configuration system**: Implement clean config management (flags + environment variables + defaults) for dev, test, and prod.
- [x] **Separate demo/seed data**: Prevent production startup from reseeding demo records; provide explicit seed commands.
- [x] **Consistent error handling & logging**: Add structured JSON logging (slog), unique request IDs, and `/healthz` / `/readyz` endpoints.
- [x] **Versioned API contracts**: Define `/api/v1` routes and backwards-compatibility rules.
- [x] **Coding standards & CI baseline**: Establish linters (`golangci-lint`), automated test pipelines, and PR checks.

> **Exit Gate**: A new developer can clone the repository, configure it, run automated tests, and start the application using documented commands.

---

## Phase 2 — Build Real Security

**Goal:** Nobody can access business information or perform actions merely by knowing an endpoint or guessing an ID.

### Security Checklist (13/13)
- [x] Implement user registration, login, logout, account recovery, and secure session management.
- [x] Choose an identity provider or implement reviewed authentication (avoid custom cryptography).
- [x] Add role-based access control (RBAC) for Owner, Admin, Finance, Operations, HR, Employee, and Read-Only.
- [x] Add multi-tenant / business-entity isolation to all records and database queries.
- [x] Enforce permissions strictly in the backend, not just in UI presentation.
- [x] Authenticate MCP clients and apply granular permissions per tool.
- [x] Protect approvals, payroll, financial entries, and inventory movements with explicit high-impact permissions.
- [x] Configure HTTPS/TLS, secure HTTP-only cookies, strict CORS, CSRF defense, and safe binding addresses.
- [x] Validate all inputs: request size limits, rate limits, timeouts, and pagination controls.
- [x] Store secrets in environment-specific secret stores; zero credentials in git.
- [x] Implement secure password/passkey handling via standard hashing (Argon2id/bcrypt).
- [x] Add account revocation, session expiration, suspicious-login alerts, and audit logging.
- [x] Define privacy policies, data retention, export, and deletion workflows.

> **Exit Gate**: Unauthenticated and unauthorised requests are rejected by the backend, and automated tests prove tenant isolation.

---

## Phase 3 — Make Business Workflows Safe and Transactional

**Goal:** AOSphere must never report that an action succeeded when the actual business operation failed.

### Action Execution Lifecycle

```mermaid
flowchart TD
    A[User or agent requests action] --> B[Authenticate identity]
    B --> C[Check role and business scope]
    C --> D[Validate input and business rules]
    D --> E{Human approval required?}
    E -- Yes --> F[Persist pending approval]
    F --> G[Authorised human reviews exact action]
    G --> H{Approved?}
    H -- No --> I[Record rejection]
    H -- Yes --> J[Execute action idempotently]
    E -- No --> J
    J --> K{Execution succeeded?}
    K -- Yes --> L[Commit state and audit event]
    K -- No --> M[Record failure and recover state]
    L --> N[Notify caller and publish event]
    M --> N
```

### Workflow Checklist (16/16)
- [x] Create a central application-service layer used identically by REST endpoints and MCP tools.
- [x] Implement one central policy engine for financial thresholds, approvals, roles, and prohibited actions.
- [x] Define deterministic state machines for purchase orders, invoices, deals, tasks, shifts, and approvals.
- [x] Link every approval to the exact action, payload hash, entity, and version being approved.
- [x] Implement explicit approval lifecycle states: `pending` → `approved`/`rejected` → `executing` → `completed`/`failed`.
- [x] Ensure approval decisions are atomic and conditional on remaining in `pending` state.
- [x] Add idempotency keys (`Idempotency-Key` header / parameter) to all write operations.
- [x] Explicit handling for rejection, cancellation, and error paths without orphaned side effects.
- [x] Calculate all PO totals, line items, discounts, and taxes strictly on the server.
- [x] Validate referenced entities (supplier, customer, SKU, employee) before transaction commit.
- [x] Prevent duplicate purchase orders and automatic reorders.
- [x] Enforce database transactions (`BEGIN...COMMIT`) across all multi-record writes.
- [x] Optimistic concurrency control (version checks) to prevent race conditions and lost updates.
- [x] Bounded retry policies for transient errors without duplicate external side effects.
- [x] Reconciliation background jobs detecting mismatched approvals, orders, and executions.
- [x] Clear delineation between autonomous agent actions vs. actions requiring human confirmation.

> **Exit Gate**: Every high-impact action has enforceable permissions, validated inputs, a defined state transition, durable audit trails, and a safe retry/failure path.

---

## Phase 4 — Make Data and Financial Records Trustworthy

**Goal:** The business database must remain correct under concurrent requests, crashes, and retries.

### Data & Finance Checklist (16/16)
- [x] Versioned database migrations with upgrade and rollback scripts (`internal/db/migration.go`).
- [x] Enforce foreign keys, unique constraints, check constraints, and indexes.
- [x] Tenant/business-entity ownership on every table (`org_id` scoped).
- [x] Store money in integer minor units (cents math in `internal/finance/money.go`)—never binary floating point.
- [x] Explicit currency codes, exchange rates, and half-up rounding rules.
- [x] Configurable, effective-dated tax calculation rules.
- [x] Proper double-entry general ledger with debits == credits invariant (`internal/ledger/ledger.go`).
- [x] Chart of accounts, journal entries, posting rules, accounting periods, and reversals.
- [x] Linked audit trails connecting invoices, payments, credit notes, and ledger entries.
- [x] Sequential, collision-free invoice and PO numbering (`internal/finance/sequence.go`).
- [x] Formal financial states: `draft`, `issued`, `paid`, `overdue`, `cancelled`, `reversed`.
- [x] Inventory reservations, real-time available stock, movement logs, and physical counts.
- [x] Atomic transactional outbox for event publishing (`outbox_events`).
- [x] Automated, encrypted daily backups with point-in-time recovery (`internal/backup/backup.go`).
- [x] Regular automated restore verification drills (`PRAGMA integrity_check`).
- [x] Data-at-rest encryption and strict database connection boundaries.

> **Exit Gate**: All money is represented in integer cents; every financial event posts a balanced double-entry journal entry ($\sum \text{Debits} == \sum \text{Credits}$); sequential IDs never collide; backups are taken online and verified without human intervention.

---

## Phase 5 — Turn MCP into a Reliable Agent Platform

**Goal:** Agents should be capable of useful work without being able to bypass the BOS's business rules.

### Agent Platform Checklist (0/15)
- [ ] Comprehensive MCP registry detailing tools, resources, prompts, permissions, and side effects.
- [ ] Strongly typed JSON schemas for all tool inputs and outputs.
- [ ] Route all agent tool calls directly through the core application services.
- [ ] Per-tool and per-entity permission enforcement.
- [ ] Clear categorization: Read-only, Reversible mutations, High-impact mutations.
- [ ] Durable execution IDs, idempotency keys, execution deadlines, and cancellation tokens.
- [ ] Persistent agent run history: tool inputs, results, execution duration, and error traces.
- [ ] Treat all external inputs (reviews, emails, web content) as untrusted; guard against prompt injection.
- [ ] Enforce factual validation; prevent hallucinated IDs or fabricated financial amounts.
- [ ] Context isolation: prevent agents from accessing keys or cross-tenant records via arguments.
- [ ] Rate limits, token/cost budgets, and concurrency controls for agent actions.
- [ ] Approval escalation, timeouts, human handoff, and an emergency global kill switch.
- [ ] Reliable event streaming with reconnect buffers and message replay.
- [ ] Versioned MCP tool contracts with integration tests.
- [ ] Automated adversarial testing suite simulating rogue agent tool executions.

> **Exit Gate**: Agents cannot bypass business rules or permissions applied to humans; all agent actions are traceable, stoppable, and safe to retry.

---

## Phase 6 — Finish the Actual Business Product

**Goal:** Deliver full business lifecycles across core modules, not merely basic CRUD tables.

### Product Completeness Checklist (0/15)
- [ ] Organization onboarding, company profile, and initial setup wizard.
- [ ] Team management, invitations, departments, and granular employee access.
- [ ] ERP Lifecycles: PO creation → Supplier approval → Goods receipt → Supplier invoice → Payment.
- [ ] CRM Lifecycles: Lead capture → Deal stages → Quote generation → Order conversion → Invoicing.
- [ ] Finance Lifecycles: Invoicing → Delivery → Payment capture → Bank reconciliation → Reporting.
- [ ] HR Lifecycles: Attendance, time-off requests, manager approvals, timesheets, payroll lock.
- [ ] Task Management: Assignment, priorities, dependencies, audit logs, completion sign-off.
- [ ] Document repository: Access controls, version tracking, and retention rules.
- [ ] Global search across records with permission-aware filtering and CSV/PDF export.
- [ ] Notification engine: In-app badges, email/webhook alerts, and notification preferences.
- [ ] Complete UI states: Loading skeletons, empty states, validation errors, conflict dialogs.
- [ ] Full accessibility (WCAG 2.1 AA compliant keyboard navigation and semantic HTML).
- [ ] Activity timelines and business-level audit log viewers in the UI.
- [ ] Verified external integrations with real provider credentials and error handling.
- [ ] Localization: Multi-currency, time zones, date formats, and regional tax settings.

---

## Phase 7 — Build a Complete Automated Testing System

**Goal:** Changes must never silently break business policies or previously working workflows.

### Regression Scenarios Matrix

| Scenario | Required Invariant / Outcome |
| :--- | :--- |
| High-value PO created by Agent | Remains in `pending` state until authorized human approves |
| Concurrent approvals on same PO | Exactly one approval commits; second receives conflict error |
| Incorrect math / tax submitted | Server rejects request before writing to database |
| Network retry on invoice generation | Idempotency key prevents duplicate invoice and double billing |
| Inventory reorder threshold reached | Exactly one active reorder created; subsequent triggers de-duplicated |
| Cross-tenant ID query or mutation | Returns HTTP 403 / 404; zero data leaked |
| DB write fails during multi-step operation | Full transaction rollback; zero orphaned records |
| External webhook / API timeout | Graceful retry via outbox; operation marked pending, not failed |
| Malicious input in review / note | Sanitized or encoded; zero XSS execution |
| Prompt injection in input text | Sanitized; agent tools reject unauthorized operations |

### Testing Checklist (0/15)
- [ ] Unit tests for business calculations, state transitions, and tax/accounting policies.
- [ ] Repository tests using isolated SQLite in-memory or temporary databases.
- [ ] REST API integration tests covering status codes, validation, and permissions.
- [ ] MCP protocol tests validating JSON-RPC schemas, tools, and error payloads.
- [ ] Full end-to-end workflow integration tests.
- [ ] Concurrency and race condition testing (`go test -race`).
- [ ] Invariant property tests (e.g. accounting balances, inventory conservation).
- [ ] Security testing: Injection, auth bypass, tenant leakage, parameter tampering.
- [ ] Database migration rollback and upgrade validation tests.
- [ ] Automated backup restore verification tests.
- [ ] Deterministic testing clock and reproducible test seed fixtures.
- [ ] Performance benchmarks and load tests under concurrent SSE and API traffic.
- [ ] CI pipeline (`.github/workflows/ci.yml`) running lint, build, test with coverage.
- [ ] Branch protection rules requiring CI pass and PR reviews on `main`.
- [ ] Staging automated smoke test suite.

---

## Phase 8 — Production Infrastructure, Monitoring & Operations

### Production Architecture

```mermaid
flowchart TD
    Client["Browser and authorised MCP clients"] --> Ingress["HTTPS ingress"]
    Ingress --> Auth["Authentication and access control"]
    Auth --> Adapters["Go REST and MCP adapters"]
    Adapters --> Services["Shared application services"]
    Services --> Policy["Policy and approval engine"]
    Services --> DB[("Transactional database")]
    Services --> Outbox[("Transactional outbox")]
    Outbox --> Workers["Background workers"]
    Workers --> ExtAPIs["External provider APIs"]
    Services --> Audit[("Audit and execution history")]
    Services --> Obs["Logs, metrics and tracing"]
    DB --> Backups["Encrypted backups"]
```

### Operations Checklist (0/16)
- [ ] Environment separation: Development, Staging, and Production configs.
- [ ] Pinned CI/CD release builds and container packaging (minimal scratch/distroless Dockerfile).
- [ ] Reverse proxy (Caddy / Nginx / Traefik) handling TLS termination and security headers.
- [ ] SQLite WAL optimization: checkpoints, connection pooling (`MaxOpenConns(1)` for writes), monitoring.
- [ ] Standardized liveness (`/healthz`) and readiness (`/readyz`) health checks.
- [ ] Structured JSON logging with trace/request IDs and secret sanitization.
- [ ] Prometheus metrics endpoint (`/metrics`): request latencies, error counts, DB stats.
- [ ] Business metrics: pending approvals, agent tool failures, failed outbox jobs.
- [ ] Alerting rules and escalation procedures.
- [ ] Durable background worker queue with exponential backoff and dead-letter queues.
- [ ] Graceful process shutdown handling `SIGTERM` / `SIGINT`.
- [ ] Automated container and vulnerability scanning (Govulncheck / Dependabot).
- [ ] Zero-downtime database migration strategy.
- [ ] Disaster recovery playbook with target RTO (< 1 hour) and RPO (< 15 minutes).
- [ ] Load and endurance testing.
- [ ] Formal incident postmortem and runbook documentation.

---

## Phase 9 — Launch Readiness & Governance

### Launch Checklist (0/14)
- [ ] Production support on-call schedule and operational ownership.
- [ ] Completed threat model and architecture security assessment.
- [ ] Third-party penetration testing.
- [ ] Legal compliance review (GDPR, SOC2 readiness, regional tax rules).
- [ ] Terms of Service and Privacy Policy documentation.
- [ ] Customer support channels and escalation paths.
- [ ] Phased customer pilot with non-critical business workflows.
- [ ] Business record audit vs. physical financial reconciliation.
- [ ] Defined SLOs (99.9% availability, p95 latency < 200ms).
- [ ] Public status page.
- [ ] Tiered usage quotas and rate limits.
- [ ] Automated monthly patch schedule.
- [ ] Agent behavior and failure retrospective framework.
- [ ] API and MCP deprecation and versioning policies.

---

## Recommended Sprint Breakdown

```
Sprint 1: Stability   ──> Canonical Go runtime, configuration, health checks, baseline CI
Sprint 2: Security    ──> Auth, RBAC, tenant isolation, secure sessions, input sanitization
Sprint 3: Integrity   ──> Service layer, policy engine, approvals, idempotency, transactions
Sprint 4: Data        ──> Migrations, double-entry ledger, inventory constraints, backups
Sprint 5: Reliability ──> MCP tool contracts, durable jobs, agent execution logging, guardrails
Sprint 6: Release     ──> Product workflows, automated regression tests, deployment setup
```
