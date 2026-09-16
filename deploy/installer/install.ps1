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

# --- Stop any existing install FIRST, so the running agent.exe doesn't lock the
# file we're about to overwrite (fixes "process cannot access ... it is in use").
if (Get-Service $ServiceName -ErrorAction SilentlyContinue) {
    Stop-Service $ServiceName -Force -ErrorAction SilentlyContinue
    sc.exe delete $ServiceName | Out-Null
}
if (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue) {
    Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false
}
Get-Process agent -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 2

# Binary in Program Files.
Copy-Item -Force (Join-Path $here "agent.exe") (Join-Path $InstallDir "agent.exe")
# CA + enrollment config in the data dir.
Copy-Item -Force (Join-Path $here "ca.crt") (Join-Path $DataDir "ca.crt")
# Write UTF-8 WITHOUT a BOM: Set-Content -Encoding UTF8 on Windows PowerShell 5.1
# prepends a BOM that Go's JSON parser rejects.
$agentJson = @{
    server_url   = $ServerUrl
    data_dir     = ($DataDir -replace '\\','/')   # forward slashes: valid JSON, Go accepts on Windows
    enroll_token = $EnrollToken
} | ConvertTo-Json
[System.IO.File]::WriteAllText((Join-Path $DataDir "agent.json"), $agentJson, (New-Object System.Text.UTF8Encoding($false)))

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
New-Service -Name $ServiceName -BinaryPathName "`"$InstallDir\agent.exe`"" `
    -DisplayName "SystemCheck Agent" -StartupType Automatic `
    -Description "Transparent endpoint monitoring agent (company policy applies)." | Out-Null
# Don't let a start hiccup abort the rest of setup (the session-helper task
# below must still be registered). Report status at the end instead.
try { Start-Service $ServiceName -ErrorAction Stop }
catch { Write-Warning "Service did not start yet: $($_.Exception.Message). Check C:\ProgramData\SystemCheck\agent.log" }

# --- Session helper task (per logged-in user: screenshots + foreground) --
# Runs agent.exe -session-agent at each user's logon, in their interactive
# session, as that (non-elevated) user. This is what keeps screenshots and app
# usage working while the service handles the privileged collectors.
# Launch the helper via a hidden VBScript shim so no console window flashes on
# the user's screen at logon (the consent dialog still shows normally).
$vbsPath = Join-Path $InstallDir "run-session.vbs"
$vbs = 'CreateObject("Wscript.Shell").Run """' + (Join-Path $InstallDir "agent.exe") + '"" -session-agent", 0, False'
[System.IO.File]::WriteAllText($vbsPath, $vbs, (New-Object System.Text.ASCIIEncoding))
$action    = New-ScheduledTaskAction -Execute "wscript.exe" -Argument "`"$vbsPath`""
$trigger   = New-ScheduledTaskTrigger -AtLogOn
# BUILTIN\Users (S-1-5-32-545): the task runs for whoever logs on, as that user.
$principal = New-ScheduledTaskPrincipal -GroupId "S-1-5-32-545" -RunLevel Limited
$settings  = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries `
    -ExecutionTimeLimit ([TimeSpan]::Zero) -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1)
Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
    -Principal $principal -Settings $settings -Description "SystemCheck user-session helper (screenshots, app usage)." | Out-Null

$svcState  = (Get-Service $ServiceName -ErrorAction SilentlyContinue).Status
$taskState = (Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue).State
Write-Host "SystemCheck installed:"
Write-Host "  service '$ServiceName' : $svcState  (privileged collectors)"
Write-Host "  task    '$TaskName' : $taskState  (session helper: screenshots + apps)"
Write-Host "NOTE: staff are shown a consent notice on first logon; collection stays off until accepted."
Write-Host "The session helper starts at the user's next logon. To start it now without"
Write-Host "re-logon, run (as the signed-in user):  Start-ScheduledTask -TaskName $TaskName"
