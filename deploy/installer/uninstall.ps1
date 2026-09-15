# Removes the SystemCheck agent (service, session-helper task, files). Admin.
param(
    [string]$InstallDir = "$env:ProgramFiles\SystemCheck",
    [string]$DataDir    = "$env:ProgramData\SystemCheck",
    [switch]$KeepData
)
$ErrorActionPreference = "SilentlyContinue"
$ServiceName = "SystemCheckAgent"
$TaskName    = "SystemCheckAgentSession"

if (Get-ScheduledTask -TaskName $TaskName) { Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false }
if (Get-Service $ServiceName) {
    Stop-Service $ServiceName -Force
    sc.exe delete $ServiceName | Out-Null
}
Start-Sleep -Seconds 1
Remove-Item -Recurse -Force $InstallDir
if (-not $KeepData) { Remove-Item -Recurse -Force $DataDir }
Write-Host "SystemCheck removed."
