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

## Release signing & agent updates

Sign each agent release with an ed25519 key held offline. Publish the version,
download URL, and base64 signature via `SC_AGENT_LATEST_VERSION` /
`SC_AGENT_DOWNLOAD_URL` / `SC_AGENT_SIGNATURE_B64`; the server advertises them at
`GET /v1/agent-version` (mTLS). A future agent self-updater will download the named
release, verify the signature against a pinned public key baked into the agent, and
only then swap the binary and restart the service. The binary swap + service
restart is intentionally deferred (Phase 5): it is OS-specific and unsafe to ship
without a real Windows test host.

## Monitoring the monitor

Watch the server logs for purge summaries and ingest/alert errors. Review the
audit log (dashboard, auditor/admin) regularly — it records every view and export
of monitored data.
