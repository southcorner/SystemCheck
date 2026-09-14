# SystemCheck — Threat Model

SystemCheck collects highly sensitive data (screenshots, activity, network metadata)
about identifiable people. The system itself is therefore a high-value target and is
designed defensively.

## Assets

- Screenshots and activity records of monitored staff.
- Admin credentials and MFA secrets.
- Agent client certificates and the enrollment secret.
- The audit log (integrity of "who watched whom").

## Trust boundaries

1. **Endpoint ↔ server** — the network between agents and the server is untrusted.
2. **Admin browser ↔ server** — the dashboard is served over TLS to authenticated
   admins only.
3. **Server ↔ datastores** — Postgres and MinIO sit on an isolated network segment.
4. **Operators ↔ data** — even authorized admins are constrained by RBAC and audited.

## Principal threats and mitigations

| Threat | Mitigation |
|---|---|
| Eavesdropping / MITM on agent traffic | **mTLS**; agents present enrolled client certs, server pins its CA |
| Rogue/spoofed agent injecting data | Enrollment tokens are single-use, short-lived; client cert required for all data endpoints |
| Stolen admin password | **Argon2id** hashing + mandatory **TOTP MFA**; session expiry; lockout on brute force |
| Over-broad admin access | **RBAC** (Admin/Auditor/Viewer); optional **dual-approval** to view an individual's screenshots |
| Insider misuse of the tool | **Append-only audit log** of every view/export; Auditor role can review it; admins cannot delete audit rows |
| Data at rest theft (disk/backups) | Encryption at rest for DB volume + MinIO; app-level encryption of screenshot blobs |
| Excessive/indefinite retention | Configurable **retention with automatic purge**; data minimization (blur, exclusions) |
| Tampered agent binary / update channel | **Signed binaries**; signature verified before auto-update install |
| Server compromise → mass exfil | Network isolation, least-privilege service accounts, secrets in a vault/`.env` not code, backups, rate limiting |
| Capturing genuinely private data | Per-employee **exclusions** (banking/health domains, personal apps), region blur, clipboard off by default, notice + consent |
| Legal exposure | Consent records gate agent activation; published policy; DSAR/export support (`docs/PRIVACY_AND_CONSENT.md`) |

## Secrets

- Enrollment secret, TLS CA private key, DB password, MinIO keys, SMTP/webhook creds,
  and the screenshot encryption key live in `deploy/server.env` (or a secrets
  manager), never in the repo. `server.env.example` documents the names only.

## What SystemCheck deliberately does NOT do

- No stealth/hidden mode; the endpoint indicator is always visible.
- No anti-forensics or tamper-hiding beyond ordinary service hardening.
- No TLS interception or full-content keystroke capture.
- No deployment path for devices the operator does not own.

## Residual risks to accept explicitly

- A determined admin with view rights can still see permitted data; the control is
  audit + least privilege + dual-approval, not prevention.
- Filenames for network uploads cannot be captured without TLS interception (out of
  scope); only volume + process attribution is available, correlated heuristically
  with filesystem events.
