# SystemCheck agent — production install

The agent is deployed as **two cooperating parts** so every feature keeps
working:

- a **Windows service** (`SystemCheckAgent`, runs as SYSTEM in session 0) — does
  enrollment, policy, uploads, and the privileged collectors: DNS, netflow,
  USB, installs, device posture, security log, file watch;
- a **user-session helper** (`agent.exe -session-agent`, launched by a per-user
  logon task) — does the desktop-bound collectors that only work inside the
  logged-in session: **screenshots** and **foreground app usage**.

A single service can't do both: screenshots/foreground need the interactive
desktop, while the ETW/WMI/event-log collectors need SYSTEM. The two parts hand
off through small files in the (user-writable) `spool` dir — the helper holds no
server credentials.

## Install (per machine, as Administrator)

Copy the zip contents to the machine and run:

```powershell
.\install.ps1 -ServerUrl https://<server>:8443 -EnrollToken <reusable-token>
```

This installs the binary to `%ProgramFiles%\SystemCheck`, writes `ca.crt` +
`agent.json` to `%ProgramData%\SystemCheck` (locked down so users can't read the
key/token), starts the service, and registers the logon task. Use a **reusable**
enrollment token so one package works across the fleet.

Deploy at scale by running `install.ps1` from your RMM/Intune/GPO startup script.

## Identity & consent

- The session helper reports the **logged-in Windows user** (`DOMAIN\user`); the
  server attributes the machine to that real person.
- On first logon each user sees a **consent notice**. Collection stays **off**
  until they accept; acceptance is recorded server-side per user. A visible
  indicator shows when monitoring is active.

## Uninstall

```powershell
.\uninstall.ps1          # add -KeepData to preserve %ProgramData%\SystemCheck
```

## Files in the package

| File | Purpose |
|------|---------|
| `agent.exe` | the agent (service + `-session-agent` helper in one binary) |
| `ca.crt` | dev CA the agent trusts for the server's TLS |
| `install.ps1` / `uninstall.ps1` | setup / removal |
