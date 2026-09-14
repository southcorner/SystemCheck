# SystemCheck — Security Review Checklist

Run before each release. Mapped to `docs/THREAT_MODEL.md`. Also run the repo's
`security-review` skill against the branch diff and resolve findings.

## Transport & authentication

- [ ] Agent↔server is mTLS; agent endpoints reject a missing/unknown client cert
      (`server/internal/api/middleware.go:agentAuth`).
- [ ] Enrollment tokens are single-use and expire (`store.ConsumeEnrollmentToken`).
- [ ] Dashboard is TLS-only, modern ciphers, `MinVersion` TLS 1.2+.
- [ ] Passwords hashed with argon2id; login is constant-time on the compare.
- [ ] MFA (TOTP) required once enrolled; sessions expire; MFA-gate on all data routes.

## Authorization & accountability

- [ ] RBAC enforced on every admin route (`requireRole`); user management is admin-only.
- [ ] Last-admin and self-disable guards in place (`handlers_users.go`).
- [ ] Dual-approval, when enabled, blocks screenshot list + image without an active
      approval, and an approver cannot be the requester (`store.ApproveViewRequest`).
- [ ] Every view/export/mutation writes an audit row; audit log is append-only in prod
      (no UPDATE/DELETE granted to the app role).

## Data protection & minimization

- [ ] Screenshots encrypted at rest; `SC_SCREENSHOT_KEY` set and backed up.
- [ ] DB + object store on encrypted volumes, isolated network.
- [ ] Retention windows configured; purge job runs and actually deletes blobs + rows.
- [ ] Policy exclusions (domains/processes) honored; clipboard off; consent gate active
      before any collection (`store.EffectivePolicy` sets `active` from consent).

## Input handling

- [ ] Ingest bodies size-limited; unknown event kinds stored as JSONB, not executed.
- [ ] External strings (event data, comments, CI text) treated as data everywhere.
- [ ] No secrets in the repo; `deploy/server.env` and `*.pem`/`certs/` gitignored.

## Supply chain & release

- [ ] Agent binaries signed (ed25519); `/v1/agent-version` serves the signature.
- [ ] Production agent shipped as a code-signed MSI.
- [ ] Dependencies pinned; `go.sum` committed; `npm` lockfile committed.

## Privacy/legal

- [ ] Employee notice published and consent records present before activation.
- [ ] DSAR/export path works; retention matches the published policy.
