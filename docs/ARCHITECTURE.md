# SystemCheck — Architecture

## Goals

- Monitor company-owned **Windows** endpoints used by designers and accountants.
- Core signals: randomized screenshots, apps used, download/upload activity
  (filenames + sizes where obtainable), domains accessed.
- Security-monitoring signals: USB/removable media, print jobs, device posture,
  security event log, new installs, DLP, anomaly alerts.
- On-prem, self-hosted. Transparent to staff, consent-gated, retention-bounded,
  fully audited.

## Non-goals

Stealth/covert mode, anti-detection/anti-forensics, full-content keystroke logging,
TLS interception, deployment to devices the operator does not own, or monitoring
without notice/consent.

## Components

### Agent (`agent/`)

A Windows Service (`golang.org/x/sys/windows/svc`) built as a single static binary.
Structure:

- **collectors** — one package per signal. Each implements a small `Collector`
  interface (`Name()`, `Start(ctx, emit)`). Windows-specific implementations live
  behind `//go:build windows`; non-Windows stubs keep the tree cross-compilable and
  unit-testable.
  - `screenshot` — multi-monitor capture (GDI BitBlt via `kbinani/screenshot`),
    random interval within `[min,max]`, WebP/JPEG compression, optional downscale and
    region blur.
  - `foreground` — foreground window + process, active-time aggregation, idle-gated
    (`GetLastInputInfo`).
  - `dns` — ETW `Microsoft-Windows-DNS-Client`, domain + process attribution.
  - `netflow` — ETW `Microsoft-Windows-Kernel-Network`, per-process byte counts.
  - `fswatch` — `fsnotify` on Downloads + configured folders → filename + size.
  - (later) `usb`, `printjobs`, `posture`, `eventlog`, `installs`.
- **spool** — embedded store (bbolt) that buffers events + screenshot blobs when the
  server is unreachable; drains with exponential backoff.
- **transport** — mTLS HTTPS client; batches events, gzip-compresses, posts to
  `/v1/ingest`; pulls policy from `/v1/policy`.
- **enroll** — one-time enrollment-token exchange for a client certificate.
- A visible **tray/notice indicator** so the user knows monitoring is active.

Policy (intervals, enabled collectors, redaction, retention, exclusions) is fetched
from the server, cached locally, and re-fetched periodically.

### Server (`server/`)

Go HTTP service. Packages:

- `config` — env-driven configuration.
- `store` — PostgreSQL (pgx) access + embedded SQL migrations; TimescaleDB
  hypertables for high-volume event/metric tables; MinIO (S3) client for screenshots.
- `auth` — admin accounts, password hashing (argon2id), TOTP MFA, sessions, RBAC
  (Admin / Auditor / Viewer).
- `ingest` — validates enrolled client certs, accepts event batches + screenshot
  blobs, writes to store.
- `policy` — CRUD for monitoring policies, assignment to machines/groups, agent pull.
- `report` — query endpoints backing the dashboard (timeline, app usage, domains,
  transfers, events).
- `audit` — append-only log of every admin action and every view/export of monitored
  data.
- `alert` — rule evaluation → email/webhook notifications.
- `api` — HTTP routing, middleware (auth, audit, rate-limit, request logging).

### Dashboard (`dashboard/`)

Vite + React + TypeScript SPA served to admins over TLS. Views: machines + health,
screenshot timeline, app-usage reports, domains, data transfer, security events,
alerts, policy editor, RBAC/MFA management, audit-log viewer, consent management.

## Data flow

1. Agent enrolls with a one-time token → receives a client certificate.
2. Agent pulls its policy; starts enabled collectors.
3. Collectors emit events; large blobs (screenshots) are spooled locally.
4. Transport batches + gzips events and posts to `/v1/ingest` over mTLS.
5. Server authenticates the client cert, writes events to Postgres/Timescale and
   screenshots to MinIO, evaluates alert rules.
6. Admins view data in the dashboard; every view is written to the audit log.

## Agent↔server contract

HTTPS/JSON over mTLS (no protoc dependency). See `proto/agent.md` for the endpoint
and payload schemas. Endpoints:

- `POST /v1/enroll` — token → client certificate (bootstrap; server-auth TLS only).
- `GET  /v1/policy` — current policy for the calling agent (mTLS).
- `POST /v1/ingest` — batched events + screenshot blobs (mTLS, gzip).
- `POST /v1/heartbeat` — liveness + agent version/health (mTLS).

## Storage model

- `machines`, `agents`, `enrollment_tokens`
- `policies`, `policy_assignments`
- `events` (Timescale hypertable; typed by `kind`, JSONB `data`)
- `screenshots` (metadata row + MinIO object key)
- `app_usage` (rolled-up active-time per app/window)
- `admin_users`, `sessions`, `audit_log`
- `alerts`, `alert_rules`
- `consent_records`

See `server/migrations/0001_init.sql`.

## Security model

See `docs/THREAT_MODEL.md`. Highlights: mTLS agent↔server, TLS-only dashboard,
encryption at rest for screenshots + DB, RBAC + MFA, append-only audit log,
data minimization + retention purge, signed agent binaries + secure auto-update.

## Roadmap

- **Phase 0 — Foundations** *(this scaffold)*: monorepo, docs, contract, DB schema,
  docker-compose, server skeleton, agent skeleton, dashboard skeleton.
- **Phase 1 — MVP**: screenshots + foreground-app usage end-to-end; admin login +
  MFA; basic policy; offline buffering.
- **Phase 2 — Network & files** *(implemented)*: DNS domains (ETW), per-process
  transfer volume (ETW), Downloads watcher (filenames+sizes) + browser download
  history; domain block-list and large-upload alerts; dashboard domains/transfers/
  downloads tabs and an alerts view. DNS/netflow are Windows-only (ETW) with
  cross-platform no-op stubs; fswatch is cross-platform.
- **Phase 3 — Security monitoring** *(implemented, except security-log events)*: USB
  removable-media insert/remove, print jobs, device posture (BitLocker/Defender/
  firewall/patch), new software installs; DLP keyword rules, USB-insert and
  new-install alerts; dashboard security tab. USB/printjobs/posture/installs are
  Windows-only with cross-platform no-op stubs. Windows Security event-log ingestion
  (logon/failed-logon/privilege-escalation) is deferred to a follow-up as it needs
  the wevtapi bindings rather than the ETW path used here.
- **Phase 4 — Hardening & ops** *(implemented; full auto-update = Phase 5)*: automatic
  retention purge, admin user management + full RBAC, dual-approval screenshot viewing,
  Windows Security event-log collector (`seclog`, closing the Phase 3 deferral) +
  failed-logon alert, `GET /v1/agent-version` release advertisement, backup script,
  Windows service installer, and the operations + security-review docs
  (`docs/OPERATIONS.md`, `docs/SECURITY_REVIEW.md`).
- **Phase 5 — Deferred**: binary auto-update (download-verify-swap-restart, using the
  ed25519 signing already advertised), an automated WiX MSI build in CI, HA/replication,
  and SIEM/webhook export.
