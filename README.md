# SystemCheck

Self-hosted endpoint monitoring for company-owned **Windows** machines, aimed at
small teams of designers and accountants. It captures screenshots on randomized
intervals, application usage, download/upload activity, and accessed domains, plus a
set of security-monitoring features — and surfaces everything in an on-prem admin
dashboard.

> **This is transparent workplace monitoring, not covert surveillance.**
> SystemCheck is designed for company-owned devices with employee notice and
> consent, bounded data retention, and full audit of who views what. It deliberately
> ships **no** stealth, anti-detection, or covert-capture features, and the agent
> shows a visible indicator on the endpoint. Deploying it on devices you do not own,
> or without notifying the people using them, is out of scope and unsupported — and
> is illegal in many jurisdictions. Read `docs/PRIVACY_AND_CONSENT.md` before you
> deploy.

## Components

| Path         | What it is                                                        |
|--------------|-------------------------------------------------------------------|
| `agent/`     | Go Windows service + collectors, one per monitored PC             |
| `server/`    | Go ingestion API + policy / report / audit / alert services       |
| `dashboard/` | React + TypeScript admin UI                                       |
| `deploy/`    | docker-compose (Postgres/TimescaleDB, MinIO), env samples         |
| `proto/`     | The agent↔server HTTPS/JSON contract, documented                  |
| `docs/`      | Architecture, threat model, privacy/consent policy               |

## Architecture at a glance

```
 Windows PC                          On-prem server (docker-compose)
┌──────────────────────┐            ┌───────────────────────────────────┐
│ agent (Windows svc)  │   mTLS     │  server (Go)                       │
│  collectors ─┐       │  HTTPS     │   /enroll /ingest /policy          │──► PostgreSQL
│  local spool ┴──────►│───────────►│   auth (RBAC+MFA) report audit     │    + TimescaleDB
│  tray indicator      │  batched   │                                    │──► MinIO (screenshots)
└──────────────────────┘  gzip      │  dashboard (React) ◄── admins      │
                                     └───────────────────────────────────┘
```

See `docs/ARCHITECTURE.md` for the full design and `docs/THREAT_MODEL.md` for the
security model.

## Status

All five roadmap phases are implemented: the endpoint agent (screenshots, apps,
DNS, network volume, downloads, USB, print, posture, installs, security log), the
server (enrollment, policy, ingestion, alerts, reporting, retention, RBAC + MFA,
dual-approval viewing, audit), the admin dashboard, and operations tooling
(backups, installer, signed auto-update, CI, HA). See `docs/ARCHITECTURE.md`.

Prerequisites: Go 1.26+, Node 20+, Docker, and `openssl` + `curl`. The full
collector set (screenshots, DNS, USB, …) only produces data on **Windows**; on
Linux/macOS the agent still enrolls, heartbeats, and pulls policy, so you can
exercise the whole pipeline, just with those collectors as no-ops.

## Quickstart — end to end

Run from the repository root. Steps 1–3 stand up the backend; 4–6 enroll an agent
and turn monitoring on; 7 shows the data.

### 1. Configure secrets

```bash
cd deploy
cp server.env.example server.env
# Generate real secrets (edit server.env and paste these in):
echo "SC_SESSION_SECRET=$(openssl rand -hex 32)"
echo "SC_ENROLL_SECRET=$(openssl rand -hex 32)"
echo "SC_SCREENSHOT_KEY=base64:$(openssl rand -base64 32)"
# Also set DB_PASSWORD, MINIO_ROOT_USER/PASSWORD, and SC_BOOTSTRAP_ADMIN_EMAIL/PASSWORD.
```

### 2. Start the datastores

```bash
docker compose --env-file server.env up -d      # Postgres/TimescaleDB + MinIO
```

### 3. Run the server

```bash
cd ../server
set -a; . ../deploy/server.env; set +a          # export the same config
go run ./cmd/server
```

On first run it generates a self-signed dev CA and TLS cert under `server/certs/`,
applies migrations, and creates the bootstrap admin. It listens on
`https://127.0.0.1:8443`. Leave it running.

Optional dashboard (new terminal): `cd dashboard && npm install && npm run dev`,
then open the printed URL and sign in with the bootstrap admin. Enrollment tokens
and consent are API operations (below); the dashboard is for viewing data, alerts,
users, and approvals.

### 4. Get an admin session and an enrollment token

Do this in a new terminal (the dev server cert is signed by `server/certs/ca.crt`,
so point `curl` at it). Fresh admins are usable before enrolling MFA; enabling MFA
is strongly recommended for real deployments (`POST /api/mfa/enroll` then
`/api/mfa/verify`).

```bash
CA=server/certs/ca.crt
BASE=https://127.0.0.1:8443

# Log in -> session token (use the bootstrap admin you set in server.env).
TOKEN=$(curl -s --cacert $CA $BASE/api/login \
  -d '{"email":"admin@example.com","password":"change-me-on-first-login"}' \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')

# Mint a one-time enrollment token, assigned to a user (here "alice").
ENROLL=$(curl -s --cacert $CA -H "Authorization: Bearer $TOKEN" \
  $BASE/api/enroll-tokens -d '{"assigned_user":"alice","group":"designers","ttl_minutes":60}' \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
echo "enrollment token: $ENROLL"
```

### 5. Record consent (monitoring stays off until you do)

The agent's policy reports `active:false` until a consent record exists for its
assigned user. This is deliberate — see `docs/PRIVACY_AND_CONSENT.md`.

```bash
curl -s --cacert $CA -H "Authorization: Bearer $TOKEN" \
  $BASE/api/consent -d '{"subject_user":"alice","method":"quickstart"}'
```

### 6. Enroll and run an agent

Provision the CA the agent should trust for enrollment, write its config, and run
it in console mode:

```bash
mkdir -p /tmp/sc-agent
cp server/certs/ca.crt /tmp/sc-agent/ca.crt         # installer ships this in production
cat > /tmp/sc-agent/agent.json <<JSON
{ "server_url": "https://127.0.0.1:8443", "data_dir": "/tmp/sc-agent", "enroll_token": "$ENROLL" }
JSON

cd agent
go run ./cmd/agent -console -config /tmp/sc-agent/agent.json
```

The agent enrolls (exchanging the token for a client certificate), prints the
visible "monitoring is ACTIVE" indicator, and starts collecting. On Windows the
screenshot and app collectors produce data immediately; on Linux they are no-ops
but the machine still appears and heartbeats.

### 7. See the data

In the dashboard, open **Machines** — `alice`'s host appears. On a Windows agent,
the tabs fill with screenshots and app usage; enable DNS, USB, and the rest by
assigning a policy (default policy captures screenshots + apps). **Alerts**,
**Approvals**, and **Users** cover the rest. Every view is written to the audit
log.

### Try more

- **Alerts**: create a rule, e.g. block a domain —
  `curl --cacert $CA -H "Authorization: Bearer $TOKEN" $BASE/api/alert-rules -d '{"name":"no-dropbox","kind":"domain_blocklist","params":{"domains":["*.dropbox.com"]}}'`
- **Dual-approval viewing**: set `SC_REQUIRE_DUAL_APPROVAL=true` and restart the
  server; viewing screenshots then needs a second admin's approval.
- **Retention**: tune `SC_RETENTION_*` days; the server purges daily.

### Cleanup

```bash
cd deploy && docker compose --env-file server.env down     # add -v to drop data
```

## Production notes

Use a real (publicly trusted) TLS certificate for the server, ship the enrollment
CA with the agent installer (`deploy/installer/`), enable MFA and
`SC_REQUIRE_DUAL_APPROVAL`, and follow `docs/OPERATIONS.md` for backups, retention,
signed auto-update, and high availability. Review `docs/SECURITY_REVIEW.md` and
`docs/PRIVACY_AND_CONSENT.md` before deploying.

## License

See `LICENSE`.
