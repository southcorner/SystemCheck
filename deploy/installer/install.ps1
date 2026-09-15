# Installs the SystemCheck agent for PRODUCTION: a SYSTEM service (privileged
# collectors) plus a per-user logon task that runs the session helper in each
# interactive session (screenshots + foreground app). Run as Administrator.
#
#   .\install.ps1 -ServerUrl https://systemcheck.corp:8443 -EnrollToken <reusable-token>
#
# Expects agent.exe and ca.crt alongside this script (the zip layout).
param(
    [Parameter(Mandatory=$true)][string]$ServerUrl,
    [Parameter(Mandatory=$true)][string]$EnrollToken,
    [string]$InstallDir = "$env:ProgramFiles\SystemCheck",
    [string]$DataDir    = "$env:ProgramData\SystemCheck"
)
$ErrorActionPreference = "Stop"
$ServiceName = "SystemCheckAgent"
$TaskName    = "SystemCheckAgentSession"
$here        = Split-Path -Parent $MyInvocation.MyCommand.Path

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir    | Out-Null
$spool = Join-Path $DataDir "spool"
New-Item -ItemType Directory -Force -Path $spool | Out-Null

# Binary in Program Files.
Copy-Item -Force (Join-Path $here "agent.exe") (Join-Path $InstallDir "agent.exe")
# CA + enrollment config in the data dir.
Copy-Item -Force (Join-Path $here "ca.crt") (Join-Path $DataDir "ca.crt")
@{
    server_url   = $ServerUrl
    data_dir     = ($DataDir -replace '\\','/')   # forward slashes: valid JSON, Go accepts on Windows
    enroll_token = $EnrollToken
} | ConvertTo-Json | Set-Content -Path (Join-Path $DataDir "agent.json") -Encoding UTF8

# --- ACLs ---------------------------------------------------------------
# Data-dir root: only SYSTEM + Administrators may read files (protects the
# client key, cert and enroll token). Users get traverse/list on FOLDERS only
# (CI, no OI) so the session helper can reach the spool but cannot read secrets.
& icacls "$DataDir" /inheritance:r /grant "*S-1-5-18:(OI)(CI)F" "*S-1-5-32-544:(OI)(CI)F" | Out-Null
& icacls "$DataDir" /grant "*S-1-5-32-545:(CI)(RX)" | Out-Null
# Spool: the unprivileged session helper writes screenshots/events + its
# session.json here, and reads runtime.json the service drops in.
& icacls "$spool" /grant "*S-1-5-32-545:(OI)(CI)M" | Out-Null

# --- Service (SYSTEM, session 0: privileged collectors) -----------------
if (Get-Service $ServiceName -ErrorAction SilentlyContinue) {
    Stop-Service $ServiceName -Force -ErrorAction SilentlyContinue
    sc.exe delete $ServiceName | Out-Null
    Start-Sleep -Seconds 2
}
New-Service -Name $ServiceName -BinaryPathName "`"$InstallDir\agent.exe`"" `
    -DisplayName "SystemCheck Agent" -StartupType Automatic `
    -Description "Transparent endpoint monitoring agent (company policy applies)." | Out-Null
Start-Service $ServiceName

# --- Session helper task (per logged-in user: screenshots + foreground) --
# Runs agent.exe -session-agent at each user's logon, in their interactive
# session, as that (non-elevated) user. This is what keeps screenshots and app
# usage working while the service handles the privileged collectors.
if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}
$action    = New-ScheduledTaskAction -Execute "$InstallDir\agent.exe" -Argument "-session-agent" -WorkingDirectory $InstallDir
$trigger   = New-ScheduledTaskTrigger -AtLogOn
# BUILTIN\Users (S-1-5-32-545): the task runs for whoever logs on, as that user.
$principal = New-ScheduledTaskPrincipal -GroupId "S-1-5-32-545" -RunLevel Limited
$settings  = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
    -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
    -Principal $principal -Settings $settings -Description "SystemCheck user-session helper (screenshots, app usage)." | Out-Null

Write-Host "SystemCheck installed:"
Write-Host "  service '$ServiceName' (running) - privileged collectors"
Write-Host "  task    '$TaskName' - session helper, starts at each user's next logon"
Write-Host "NOTE: staff are shown a consent notice on first logon; collection stays off until accepted."
Write-Host "To start the helper now without re-logon, have the signed-in user log off and back on."
