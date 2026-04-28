# Aegis Proxy MITM Setup Script for Windows
# Run as Administrator

$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectDir = Split-Path -Parent $ScriptDir

Write-Host "===================================" -ForegroundColor Cyan
Write-Host "Aegis Proxy MITM Setup (Windows)" -ForegroundColor Cyan
Write-Host "===================================" -ForegroundColor Cyan
Write-Host ""

# Check if running as Administrator
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Host "ERROR: This script must be run as Administrator" -ForegroundColor Red
    Write-Host "Right-click PowerShell and select 'Run as Administrator'" -ForegroundColor Yellow
    exit 1
}

Write-Host "[1/5] Building aegis binary..." -ForegroundColor Yellow
Set-Location $ProjectDir
go build -o aegis.exe ./cmd/aegis
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Failed to build binary" -ForegroundColor Red
    exit 1
}
Write-Host "✓ Binary built successfully" -ForegroundColor Green
Write-Host ""

Write-Host "[2/5] Generating CA certificate..." -ForegroundColor Yellow
.\aegis.exe mitm setup-ca
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Failed to generate CA certificate" -ForegroundColor Red
    exit 1
}
Write-Host "✓ CA certificate generated" -ForegroundColor Green
Write-Host ""

Write-Host "[3/5] Adding entries to hosts file..." -ForegroundColor Yellow
.\aegis.exe mitm setup-hosts
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Failed to add hosts entries" -ForegroundColor Red
    exit 1
}
Write-Host "✓ Hosts entries added" -ForegroundColor Green
Write-Host ""

Write-Host "[4/5] Installing CA to Windows Certificate Store..." -ForegroundColor Yellow
.\aegis.exe mitm setup-trust
if ($LASTEXITCODE -ne 0) {
    Write-Host "ERROR: Failed to install CA certificate" -ForegroundColor Red
    exit 1
}
Write-Host "✓ CA certificate installed" -ForegroundColor Green
Write-Host ""

Write-Host "[5/5] Starting MITM proxy..." -ForegroundColor Yellow
Start-Process -FilePath ".\aegis.exe" -ArgumentList "mitm start" -WindowStyle Hidden
Start-Sleep -Seconds 2
Write-Host "✓ MITM proxy started" -ForegroundColor Green
Write-Host ""

Write-Host "===================================" -ForegroundColor Cyan
Write-Host "Setup Complete!" -ForegroundColor Cyan
Write-Host "===================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "MITM Proxy is now running on port 8443" -ForegroundColor Green
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "1. Configure Cursor/Trae/Windsurf to use proxy:"
Write-Host "   - HTTP Proxy: http://127.0.0.1:8443"
Write-Host "   - HTTPS Proxy: https://127.0.0.1:8443"
Write-Host ""
Write-Host "2. Start the main proxy server:"
Write-Host "   .\aegis.exe start"
Write-Host ""
Write-Host "3. Test the setup:"
Write-Host "   curl -x http://127.0.0.1:8443 https://api.openai.com/v1/models"
Write-Host ""
Write-Host "To stop MITM proxy:" -ForegroundColor Yellow
Write-Host "   .\aegis.exe mitm stop"
Write-Host ""
Write-Host "To cleanup (remove hosts entries and uninstall CA):" -ForegroundColor Yellow
Write-Host "   .\aegis.exe mitm disable"
Write-Host ""
