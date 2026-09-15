# Assembles the deployable zip: agent.exe + ca.crt + install/uninstall scripts,
# plus a self-elevating install.bat so pilots can install by right-click ->
# "Run as administrator" with no typing. Run from the repo root after building
# the agent (agent\agent.exe).
#   .\deploy\installer\pack-zip.ps1 -ServerUrl https://<server>:8443 -EnrollToken <reusable-token>
param(
    [string]$ServerUrl,
    [string]$EnrollToken,
    [string]$AgentExe = "agent\agent.exe",
    [string]$CaCrt    = "server\certs\ca.crt",
    [string]$OutDir   = "deploy\installer\build"
)
$ErrorActionPreference = "Stop"
$here = "deploy\installer"
$pkg  = Join-Path $OutDir "pkg"
Remove-Item -Recurse -Force $pkg -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force $pkg | Out-Null

Copy-Item -Force $AgentExe (Join-Path $pkg "agent.exe")
Copy-Item -Force $CaCrt    (Join-Path $pkg "ca.crt")
Copy-Item -Force (Join-Path $here "install.ps1")   (Join-Path $pkg "install.ps1")
Copy-Item -Force (Join-Path $here "uninstall.ps1") (Join-Path $pkg "uninstall.ps1")
Copy-Item -Force (Join-Path $here "README.md")      (Join-Path $pkg "README.md")

# Generate a one-click installer batch. It self-elevates (UAC), unblocks the
# files, and runs install.ps1 with the baked server URL + enrollment token.
if (-not $ServerUrl)   { $ServerUrl   = "https://SET-SERVER:8443"; Write-Warning "No -ServerUrl given; edit install.bat before use." }
if (-not $EnrollToken) { $EnrollToken = "SET-ENROLL-TOKEN";        Write-Warning "No -EnrollToken given; edit install.bat before use." }
$bat = @"
@echo off
setlocal
REM ==== SystemCheck one-click installer - right-click > Run as administrator ====
REM Edit these two lines only if the server address or token changes.
set "SERVER_URL=$ServerUrl"
set "ENROLL_TOKEN=$EnrollToken"
REM ============================================================================
net session >nul 2>&1
if %errorlevel% NEQ 0 (
  echo Requesting administrator privileges...
  powershell -NoProfile -Command "Start-Process -Verb RunAs -FilePath '%~f0'"
  exit /b
)
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -Command "Get-ChildItem -LiteralPath '%~dp0' -Recurse | Unblock-File"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" -ServerUrl "%SERVER_URL%" -EnrollToken "%ENROLL_TOKEN%"
echo.
echo ==== Done. Review the output above. ====
pause
"@
# Write ASCII (no BOM) so cmd.exe reads it cleanly. Use an absolute path:
# [IO.File] resolves relative paths against the process dir, not $PWD.
$batPath = Join-Path (Resolve-Path $pkg).Path "install.bat"
[System.IO.File]::WriteAllText($batPath, $bat, (New-Object System.Text.ASCIIEncoding))

$zip = Join-Path $OutDir "SystemCheckAgent.zip"
Remove-Item -Force $zip -ErrorAction SilentlyContinue
Compress-Archive -Path (Join-Path $pkg "*") -DestinationPath $zip
Write-Host "Built $zip (server=$ServerUrl)"
