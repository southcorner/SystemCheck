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

Early scaffold. The server builds and runs against Postgres; the agent has a
cross-platform skeleton with Windows collectors behind build tags. See the roadmap
in `docs/ARCHITECTURE.md`.

## Quick start (server, for development)

```bash
cd deploy && cp server.env.example server.env   # edit secrets
docker compose up -d                             # Postgres, TimescaleDB, MinIO
cd ../server && go run ./cmd/server              # runs migrations, serves API
```

Then build the dashboard:

```bash
cd dashboard && npm install && npm run dev
```

## License

See `LICENSE`.
