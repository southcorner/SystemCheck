# Installs the SystemCheck agent as a Windows service. Run as Administrator.
# Usage:
#   .\install.ps1 -ServerUrl https://systemcheck.corp:8443 -EnrollToken <token>
param(
    [Parameter(Mandatory=$true)][string]$ServerUrl,
    [Parameter(Mandatory=$true)][string]$EnrollToken,
    [string]$InstallDir = "$env:ProgramFiles\SystemCheck",
    [string]$DataDir    = "$env:ProgramData\SystemCheck"
)
$ErrorActionPreference = "Stop"
$ServiceName = "SystemCheckAgent"

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
New-Item -ItemType Directory -Force -Path $DataDir | Out-Null

# Copy the agent binary (agent.exe expected alongside this script).
Copy-Item -Force "$PSScriptRoot\agent.exe" "$InstallDir\agent.exe"

# Write config with the one-time enrollment token (cleared after first enroll).
$config = @{ server_url = $ServerUrl; data_dir = $DataDir; enroll_token = $EnrollToken } | ConvertTo-Json
Set-Content -Path "$DataDir\agent.json" -Value $config -Encoding UTF8

# Create and start the service.
if (Get-Service $ServiceName -ErrorAction SilentlyContinue) {
    Stop-Service $ServiceName -Force
    sc.exe delete $ServiceName | Out-Null
    Start-Sleep -Seconds 2
}
New-Service -Name $ServiceName -BinaryPathName "`"$InstallDir\agent.exe`" -config `"$DataDir\agent.json`"" `
    -DisplayName "SystemCheck Agent" -StartupType Automatic `
    -Description "Transparent endpoint monitoring agent (company policy applies)."
Start-Service $ServiceName
Write-Host "SystemCheck agent installed and started."
Write-Host "NOTE: staff must be notified per your monitoring policy; the agent shows a visible indicator."

# For production, ship a code-signed MSI built with WiX in CI rather than this script.
