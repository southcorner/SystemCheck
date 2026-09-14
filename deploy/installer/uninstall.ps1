# Removes the SystemCheck agent service. Run as Administrator.
param(
    [string]$InstallDir = "$env:ProgramFiles\SystemCheck",
    [switch]$PurgeData
)
$ErrorActionPreference = "Stop"
$ServiceName = "SystemCheckAgent"
if (Get-Service $ServiceName -ErrorAction SilentlyContinue) {
    Stop-Service $ServiceName -Force -ErrorAction SilentlyContinue
    sc.exe delete $ServiceName | Out-Null
}
Remove-Item -Recurse -Force $InstallDir -ErrorAction SilentlyContinue
if ($PurgeData) { Remove-Item -Recurse -Force "$env:ProgramData\SystemCheck" -ErrorAction SilentlyContinue }
Write-Host "SystemCheck agent removed."
