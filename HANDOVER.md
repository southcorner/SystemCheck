# SystemCheck — Handover Runbook (for an agent running on the server)

You are running on the SystemCheck **server host**, with this repository cloned.
Goal: stand up the full backend and get it to a state where a Windows machine on
the LAN can be enrolled and monitored. Do the steps in order. Commands assume the
**repository root** as the working directory unless a `cd` says otherwise. Do not
commit `deploy/server.env`, `server/certs/`, or any secret.

At the end, report the summary block in step 8 to the human.

## 0. Preconditions

Check tools and versions; stop and report if any is missing:

```bash
go version        # need 1.26+
node --version    # need 20+
npm --version
docker --version  # and: docker compose version
openssl version
curl --version | head -1
```

If `docker` needs sudo on this host, prefix the compose commands with `sudo` (and
say so in your report). Determine the server's LAN IP now — you'll reuse it:

```bash
LAN_IP=$(hostname -I | awk '{print $1}'); echo "LAN_IP=$LAN_IP"
```

## 1. Generate secrets (only if not already present)

```bash
cd deploy
if [ ! -f server.env ]; then
  ADMIN_PASS=$(openssl rand -hex 12)
  cat > server.env <<EOF
SC_LISTEN_ADDR=:8443
SC_TLS_SANS=$LAN_IP
DB_USER=systemcheck
DB_PASSWORD=$(openssl rand -hex 16)
DB_NAME=systemcheck
DB_HOST=127.0.0.1
DB_PORT=5432
MINIO_ROOT_USER=systemcheck
MINIO_ROOT_PASSWORD=$(openssl rand -hex 16)
SC_S3_ENDPOINT=127.0.0.1:9000
SC_S3_BUCKET=screenshots
SC_S3_USE_SSL=false
SC_SCREENSHOT_KEY=base64:$(openssl rand -base64 32)
SC_SESSION_SECRET=$(openssl rand -hex 32)
SC_ENROLL_SECRET=$(openssl rand -hex 32)
SC_BOOTSTRAP_ADMIN_EMAIL=admin@example.com
SC_BOOTSTRAP_ADMIN_PASSWORD=$ADMIN_PASS
EOF
  echo "wrote deploy/server.env (bootstrap admin password: $ADMIN_PASS)"
else
  echo "deploy/server.env already exists; leaving it as-is"
  # Ensure the LAN IP is in SC_TLS_SANS; if not, add it and delete certs to regen.
  grep -q "SC_TLS_SANS=.*$LAN_IP" server.env || echo "ACTION: set SC_TLS_SANS=$LAN_IP in server.env and 'rm -f ../server/certs/*.crt ../server/certs/*.key'"
fi
cd ..
```

Capture the bootstrap admin password from the output — you need it in step 5 and
the report.

## 2. Start the datastores

```bash
cd deploy && docker compose --env-file server.env up -d && cd ..
# wait until both are healthy:
for i in $(seq 1 30); do
  docker compose --env-file deploy/server.env -f deploy/docker-compose.yml ps | grep -q healthy && break
  sleep 2
done
docker compose --env-file deploy/server.env -f deploy/docker-compose.yml ps
```

## 3. Build and run the server (background)

```bash
cd server
set -a; . ../deploy/server.env; set +a
go build -o systemcheck-server ./cmd/server
nohup ./systemcheck-server > /tmp/systemcheck-server.log 2>&1 &
echo $! > /tmp/systemcheck-server.pid
cd ..
```

It generates `server/certs/` (dev CA + TLS cert valid for `$LAN_IP`), runs
migrations, and creates the bootstrap admin. Wait for health:

```bash
for i in $(seq 1 30); do
  curl -sf --cacert server/certs/ca.crt https://127.0.0.1:8443/healthz && break
  sleep 1
done
echo; tail -n 20 /tmp/systemcheck-server.log
```

You should see `ok` and a log line about the bootstrap admin. If it exits with a
DB error, the datastores aren't ready — re-check step 2 and the log.

## 4. (Optional) Run the dashboard on the LAN

```bash
cd dashboard
npm install
nohup npm run dev -- --host 0.0.0.0 > /tmp/systemcheck-dash.log 2>&1 &
echo $! > /tmp/systemcheck-dash.pid
cd ..
sleep 3; tail -n 5 /tmp/systemcheck-dash.log
```

The human can then open `http://$LAN_IP:5173` and sign in with the bootstrap admin.

## 5. Create an enrollment token and record consent

```bash
CA=server/certs/ca.crt; BASE=https://127.0.0.1:8443
ADMIN_EMAIL=$(grep SC_BOOTSTRAP_ADMIN_EMAIL deploy/server.env | cut -d= -f2)
ADMIN_PASS=$(grep SC_BOOTSTRAP_ADMIN_PASSWORD deploy/server.env | cut -d= -f2)

TOKEN=$(curl -s --cacert $CA $BASE/api/login \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASS\"}" \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
[ -n "$TOKEN" ] || { echo "LOGIN FAILED"; tail /tmp/systemcheck-server.log; }

ENROLL=$(curl -s --cacert $CA -H "Authorization: Bearer $TOKEN" \
  $BASE/api/enroll-tokens -d '{"assigned_user":"testuser","group":"test","ttl_minutes":240}' \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')

curl -s --cacert $CA -H "Authorization: Bearer $TOKEN" \
  $BASE/api/consent -d '{"subject_user":"testuser","method":"handover-test"}'
echo; echo "ENROLL_TOKEN=$ENROLL"
```

## 6. Build the Windows agent

```bash
cd agent && GOOS=windows GOARCH=amd64 go build -o agent.exe ./cmd/agent && cd ..
ls -la agent/agent.exe
```

## 7. Stage the files the human copies to the Windows machine

```bash
mkdir -p /tmp/sc-handover
cp agent/agent.exe server/certs/ca.crt /tmp/sc-handover/
cat > /tmp/sc-handover/agent.json <<EOF
{ "server_url": "https://$LAN_IP:8443",
  "data_dir": "C:\\\\ProgramData\\\\SystemCheck",
  "enroll_token": "$ENROLL" }
EOF
echo "staged in /tmp/sc-handover:"; ls -la /tmp/sc-handover
```

## 8. Report back to the human

Print this summary:

- Server URL for agents: `https://<LAN_IP>:8443`
- Dashboard (if started): `http://<LAN_IP>:5173`
- Admin email / password (from `deploy/server.env`)
- Enrollment token (valid ~4h) and the assigned user (`testuser`)
- The three files staged in `/tmp/sc-handover/` (`agent.exe`, `ca.crt`, `agent.json`)
  and the Windows run instructions below
- Server log: `/tmp/systemcheck-server.log`; PID in `/tmp/systemcheck-server.pid`

Windows instructions to relay (run in an **Administrator** PowerShell on the
target machine, after copying the three staged files there):

```powershell
New-Item -ItemType Directory -Force C:\ProgramData\SystemCheck | Out-Null
Copy-Item ca.crt, agent.json C:\ProgramData\SystemCheck\
.\agent.exe -console -config C:\ProgramData\SystemCheck\agent.json
```

Confirm success by checking the dashboard **Machines** view (or
`curl -s --cacert server/certs/ca.crt -H "Authorization: Bearer $TOKEN" https://127.0.0.1:8443/api/machines`)
for the enrolled host.

## Firewall

Agents need TCP **8443** to this host (and **5173** if the human uses the dashboard
from another machine). If a firewall is active, open them, e.g.
`sudo ufw allow 8443/tcp` and `sudo ufw allow 5173/tcp`. Postgres/MinIO stay bound
to `127.0.0.1` and need no exposure.

## Stopping / cleanup

```bash
kill $(cat /tmp/systemcheck-server.pid) 2>/dev/null || true
kill $(cat /tmp/systemcheck-dash.pid) 2>/dev/null || true
cd deploy && docker compose --env-file server.env down && cd ..   # add -v to drop data
```

## Troubleshooting

- **Login returns empty token**: the admin wasn't created (check the server log) or
  the password in `server.env` differs from what you sent. Re-read both.
- **Agent can't connect / TLS error**: `$LAN_IP` isn't in the cert. Ensure
  `SC_TLS_SANS` contains it, delete `server/certs/*.crt server/certs/*.key`, and
  restart the server so the cert regenerates.
- **Port in use**: change `SC_LISTEN_ADDR` (server) or the dashboard port, and the
  agent `server_url` to match.
- **No Docker**: point `DB_*` at an existing Postgres (needs the TimescaleDB
  extension) and `SC_S3_*` at any S3-compatible store; the rest is unchanged.
- **Only non-Windows machines to test with**: the agent still enrolls and appears
  under Machines, but the screenshot/DNS/USB collectors are no-ops off Windows.
