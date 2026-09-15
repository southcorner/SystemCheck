# Assembles the deployable zip: agent.exe + ca.crt + install/uninstall scripts.
# Run from the repo root after building the agent (agent\agent.exe).
#   .\deploy\installer\pack-zip.ps1
param(
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

$zip = Join-Path $OutDir "SystemCheckAgent.zip"
Remove-Item -Force $zip -ErrorAction SilentlyContinue
Compress-Archive -Path (Join-Path $pkg "*") -DestinationPath $zip
Write-Host "Built $zip"
