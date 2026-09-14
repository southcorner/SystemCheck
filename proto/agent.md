# Agent ↔ Server contract (HTTPS/JSON over mTLS)

All endpoints are JSON over HTTPS. Every endpoint except `/v1/enroll` requires a
valid **enrolled client certificate** (mTLS). `/v1/enroll` uses server-auth TLS plus a
one-time enrollment token. Request/response bodies may be gzip-compressed
(`Content-Encoding: gzip`).

The Go types backing these payloads live in `server/internal/model` and are shared in
spirit by the agent's `transport` package.

## POST /v1/enroll

Bootstrap: exchange a one-time token for a client certificate.

Request:
```json
{
  "token": "one-time-enrollment-token",
  "hostname": "DESIGN-PC-04",
  "os": "windows",
  "os_version": "10.0.19045",
  "agent_version": "0.1.0",
  "csr_pem": "-----BEGIN CERTIFICATE REQUEST----- ..."
}
```
Response `200`:
```json
{
  "machine_id": "uuid",
  "cert_pem": "-----BEGIN CERTIFICATE----- ...",
  "ca_pem": "-----BEGIN CERTIFICATE----- ..."
}
```

## GET /v1/policy

Returns the effective policy for the calling agent (identified by client cert).

Response `200`:
```json
{
  "version": 7,
  "active": true,
  "screenshot": { "enabled": true, "min_interval_sec": 180, "max_interval_sec": 900,
                  "max_width": 1600, "format": "webp", "quality": 60, "blur_regions": [] },
  "foreground": { "enabled": true, "poll_sec": 5, "idle_threshold_sec": 120 },
  "dns":        { "enabled": false },
  "netflow":    { "enabled": false, "rollup_sec": 60 },
  "fswatch":    { "enabled": false, "folders": ["%USERPROFILE%\\Downloads"], "include_browser_history": false },
  "usb":        { "enabled": false },
  "printjobs":  { "enabled": false },
  "installs":   { "enabled": false },
  "posture":    { "enabled": false, "interval_sec": 3600 },
  "seclog":     { "enabled": false },
  "exclusions": { "domains": ["*.bank.example"], "processes": ["1password.exe"] },
  "heartbeat_sec": 60
}
```
`active:false` means consent is not on file; the agent stays idle.

## POST /v1/ingest

Batched events. Screenshot image bytes are sent as a separate multipart part or a
follow-up `POST /v1/screenshots` with the blob; metadata rides in the event.

Request:
```json
{
  "batch_id": "uuid",
  "sent_at": "2026-09-14T10:00:00Z",
  "events": [
    { "kind": "foreground", "ts": "2026-09-14T09:59:00Z",
      "data": { "process": "photoshop.exe", "title": "Untitled-1", "active_sec": 55 } },
    { "kind": "screenshot", "ts": "2026-09-14T09:59:10Z",
      "data": { "object_key": "spool/uuid.webp", "width": 1600, "height": 900, "bytes": 84213 } },
    { "kind": "dns", "ts": "2026-09-14T09:59:12Z",
      "data": { "domain": "cdn.example.com", "process": "chrome.exe" } }
  ]
}
```
Response `200`: `{ "accepted": 3, "rejected": 0 }`

## POST /v1/screenshots

Uploads a screenshot blob (referenced by `object_key`). `multipart/form-data` with
fields `object_key` and `file`. Server stores it in MinIO (encrypted) and links it to
the metadata event.

## GET /v1/agent-version

Advertises the latest agent release for update checks (mTLS). Read-only.

Response `200`:
```json
{ "version": "0.2.0", "url": "https://.../agent-0.2.0.exe", "signature": "base64-ed25519-sig" }
```
A future agent self-updater downloads `url`, verifies `signature` against a pinned
public key, then swaps the binary and restarts the service.

## POST /v1/heartbeat

```json
{ "agent_version": "0.1.0", "policy_version": 7, "queued_events": 12, "healthy": true }
```
Response `200`: `{ "ok": true }`

## Event kinds (extensible)

`foreground`, `screenshot`, `dns`, `netflow`, `download`, `upload`, `usb`,
`printjob`, `posture`, `install`, `seclog`. Unknown kinds are stored with their JSONB
`data` for forward compatibility. `seclog` data carries `{event_id, kind:logon|
failed_logon|priv, account, logon_type, source_ip, record_id}`.
