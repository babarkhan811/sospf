# S-OSPF demo runner for Windows (no `make` needed).
#   powershell -ExecutionPolicy Bypass -File scripts\demo.ps1
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location $root

$go = "go"
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    $cand = "C:\Program Files\Go\bin\go.exe"
    if (Test-Path $cand) { $go = $cand } else { throw "Go toolchain not found on PATH." }
}

Write-Host "== build ==" -ForegroundColor Cyan
& $go build ./...

Write-Host "`n== tests ==" -ForegroundColor Cyan
& $go test ./... -count=1

Write-Host "`n== multi-domain demo (planner -> best-trusted -> re-route) ==" -ForegroundColor Cyan
& $go run ./cmd/sospf-demo

Write-Host "`n== emulator: triangle topology ==" -ForegroundColor Cyan
& $go run ./cmd/sospf-emu -topo topologies/triangle.json

Write-Host "`n== resolve-intent (medical scenario) ==" -ForegroundColor Cyan
& $go run ./cmd/sospfctl resolve-intent -scenario scenarios/medical.json

Write-Host "`nDone." -ForegroundColor Green
