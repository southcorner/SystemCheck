# Builds the single-run SystemCheck agent MSI. Run from the repo root.
#
#   .\deploy\installer\build-msi.ps1 -ServerUrl https://systemcheck.corp:8443 -EnrollToken <reusable-token>
#
# Prereqs: WiX v4/v5 CLI on PATH (`dotnet tool install --global wix --version 5.0.2`)
# and a built agent binary at agent\agent.exe plus the CA at server\certs\ca.crt
# (override with -AgentExe / -CaCrt). The MSI installs agent.exe + the
# SystemCheckAgent service, and drops ca.crt + agent.json (with the enroll token)
# into %ProgramData%\SystemCheck. Use a REUSABLE enroll token for fleet-wide use.
param(
    [Parameter(Mandatory=$true)][string]$ServerUrl,
    [Parameter(Mandatory=$true)][string]$EnrollToken,
    [string]$AgentExe = "agent\agent.exe",
    [string]$CaCrt    = "server\certs\ca.crt",
    [string]$OutDir   = "deploy\installer\build"
)
$ErrorActionPreference = "Stop"
$wxs   = "deploy\installer\systemcheck-agent.wxs"
$stage = Join-Path $OutDir "stage"
New-Item -ItemType Directory -Force $stage | Out-Null

Copy-Item -Force $AgentExe (Join-Path $stage "agent.exe")
Copy-Item -Force $CaCrt    (Join-Path $stage "ca.crt")

# agent.json: forward slashes in data_dir avoid JSON backslash-escaping pitfalls;
# Go accepts them on Windows.
@{
    server_url   = $ServerUrl
    data_dir     = "C:/ProgramData/SystemCheck"
    enroll_token = $EnrollToken
} | ConvertTo-Json | Set-Content -Path (Join-Path $stage "agent.json") -Encoding UTF8

$msi = Join-Path $OutDir "SystemCheckAgent.msi"
wix build $wxs -d "Stage=$((Resolve-Path $stage).Path)" -o $msi
Write-Host "Built $msi"
Write-Host "Silent install on a target (as admin):  msiexec /i SystemCheckAgent.msi /qn"
Write-Host "For production, code-sign the MSI:       signtool sign /fd SHA256 /a $msi"
