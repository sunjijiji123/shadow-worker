$ErrorActionPreference = 'SilentlyContinue'

Get-Process -Name 'shadow-worker','shadow-worker-client','go','main' -ErrorAction SilentlyContinue | Stop-Process -Force

$conns = Get-NetTCPConnection -LocalPort 50051 -ErrorAction SilentlyContinue
foreach ($c in $conns) {
    if ($c.OwningProcess -and $c.OwningProcess -ne 0) {
        Write-Output ("Killing pid " + $c.OwningProcess + " on port 50051")
        Stop-Process -Id $c.OwningProcess -Force -ErrorAction SilentlyContinue
    }
}
Start-Sleep -Seconds 2

$still = Get-NetTCPConnection -LocalPort 50051 -ErrorAction SilentlyContinue
if ($still) {
    Write-Output "WARN: port still in use"
} else {
    Write-Output "OK: port 50051 free"
}
