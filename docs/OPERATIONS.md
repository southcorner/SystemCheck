# SystemCheck — Operations

Running SystemCheck safely against real staff data. Pairs with
`docs/PRIVACY_AND_CONSENT.md` (obligations) and `docs/THREAT_MODEL.md` (controls).

## Retention & automatic purge

The server purges data past its retention window once at startup and daily
thereafter (`server/cmd/server/main.go:runPurgeLoop`, using
`store.RetentionCutoffs` + `store.Purge`). Screenshot blobs are deleted from the
blob store before their metadata rows. Windows (days, `0` disables that class):

| Env var | Data | Default |
|---|---|---|
| `SC_RETENTION_SCREENSHOTS_DAYS` | screenshots | 30 |
| `SC_RETENTION_ACTIVITY_DAYS` | app usage, domains, transfers, downloads | 90 |
| `SC_RETENTION_SECURITY_DAYS` | usb, print, install, posture, seclog | 180 |
| `SC_RETENTION_AUDIT_DAYS` | audit log | 365 |

Tune to the legal advice in `docs/PRIVACY_AND_CONSENT.md`.

## Dual-approval to view screenshots

Set `SC_REQUIRE_DUAL_APPROVAL=true` to require a second admin's approval before
anyone can view an individual's screenshots. A viewer requests access
(dashboard Approvals view), a different admin approves, and access lasts 8 hours.
Every request, approval, and view is written to the audit log. With the flag off,
any MFA-complete admin may view (still audited).

## Admin accounts & RBAC

Roles: `admin` (everything, incl. user management, alert rules, view approvals),
`auditor` (read + audit log), `viewer` (read). The first admin is bootstrapped from
`SC_BOOTSTRAP_ADMIN_*`; manage the rest in the dashboard Users view. The server
refuses to demote or disable the last enabled admin, and admins cannot disable
themselves. Everyone must enroll TOTP MFA on first login.

## Encryption at rest

- **Screenshots**: encrypted app-side with AES-256-GCM before storage
  (`server/internal/blob/blob.go`), keyed by `SC_SCREENSHOT_KEY` (a base64 32-byte
  key). Rotate by re-keying and re-encrypting; keep the old key until purge clears
  old blobs.
- **Database & object store**: run Postgres and MinIO on encrypted volumes
  (LUKS/BitLocker/cloud KMS). Restrict them to an isolated network segment.

## Backups

`deploy/backup.sh` dumps Postgres (`pg_dump`) and copies the screenshot store; it
prints restore commands. Schedule it (cron/Task Scheduler), store backups
encrypted, and test restores. Back up secrets (`deploy/server.env`, the CA key)
separately in a secrets manager.

## TLS / mTLS certificate rotation

Agents authenticate with client certs signed by the enrollment CA
(`server/internal/pki`). Rotate the server TLS cert without touching agents. To
rotate the CA, stand up the new CA, re-enroll agents (new one-time tokens), then
retire the old CA. Client certs are issued for 1 year; re-enroll before expiry.

## Agent install / uninstall

`deploy/installer/install.ps1` installs the agent as a Windows service with a
one-time enrollment token; `uninstall.ps1` removes it. For production, ship a
**code-signed MSI** built with WiX in CI rather than the raw script.

## Release signing & agent auto-update

Signed self-update is implemented end to end.

1. Generate an ed25519 keypair once, offline: `go run ./server/cmd/scsign keygen`.
   Keep the private key offline; the public key is pinned into the agent at build
   time via `-ldflags "-X .../agent/internal/updater.PinnedPublicKey=<pubB64>"`
   (the release workflow does this from the `SC_RELEASE_PUBKEY` repo variable).
   With no key pinned, self-update is disabled entirely.
2. On each release, sign the binary:
   `go run ./server/cmd/scsign sign -key <privB64> agent.exe`.
3. Publish the version, download URL, and signature via `SC_AGENT_LATEST_VERSION` /
   `SC_AGENT_DOWNLOAD_URL` / `SC_AGENT_SIGNATURE_B64`. The server advertises them at
   `GET /v1/agent-version` (mTLS).
4. Agents check every 6 hours: if the advertised version is newer, the agent
   downloads it, verifies the signature against its pinned public key, then (on
   Windows) moves the running binary aside, writes the new one, and restarts the
   service. A verification failure aborts the update and rolls back.

`.github/workflows/release.yml` builds the pinned agent, signs it with `scsign`,
and (optionally) builds a code-signed MSI from
`deploy/installer/systemcheck-agent.wxs`.

## SIEM / webhook export

Set `SC_ALERT_WEBHOOK` to a URL to forward every alert as JSON
(`{source, machine_id, severity, message, data, timestamp}`) as it fires. Point it
at a SIEM's HTTP collector or an incident webhook. Delivery is best-effort and
never blocks ingestion.

## High availability

The server is stateless, so run two or more instances behind a load balancer that
supports client-certificate passthrough (agents use mTLS). Shared state lives in
Postgres and MinIO:

- **Postgres**: use streaming replication (a primary + one or more hot standbys)
  or a managed HA Postgres/TimescaleDB; point all server instances at the primary
  (or a connection pooler with failover).
- **MinIO**: run a distributed MinIO cluster (erasure-coded) or a managed
  S3-compatible store.
- **CA/secrets**: the enrollment CA key and `server.env` must be available to every
  instance via a secrets manager, not baked into images.
- The retention purge loop is idempotent, so multiple instances running it
  concurrently is safe (deletes simply no-op on already-purged rows).

## Monitoring the monitor

Watch the server logs for purge summaries and ingest/alert errors. Review the
audit log (dashboard, auditor/admin) regularly — it records every view and export
of monitored data.
