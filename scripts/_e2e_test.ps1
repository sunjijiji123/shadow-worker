$ErrorActionPreference = 'SilentlyContinue'

# clean first
Get-Process -Name 'shadow-worker','shadow-worker-client','go','main' -ErrorAction SilentlyContinue | Stop-Process -Force
Start-Sleep -Seconds 1

$backend = 'C:\Users\Administrator\code\1-ai\shadow-worker\backend'
$client = 'C:\Users\Administrator\code\1-ai\shadow-worker\client\build'
$qtdir = 'C:\Qt\6.11.1\msvc2022_64\bin'

# start Go service
Remove-Item "$backend\server.log","$backend\server_err.log" -ErrorAction SilentlyContinue
$svc = Start-Process -FilePath 'go' -ArgumentList 'run','cmd/shadow-worker/main.go' `
    -WorkingDirectory $backend `
    -RedirectStandardOutput "$backend\server.log" `
    -RedirectStandardError "$backend\server_err.log" `
    -PassThru -WindowStyle Hidden

# wait for service to be ready
Write-Output "Waiting for Go service..."
$ready = $false
for ($i = 0; $i -lt 30; $i++) {
    Start-Sleep -Seconds 1
    $t = Test-NetConnection -ComputerName 127.0.0.1 -Port 50051 -InformationLevel Quiet -WarningAction SilentlyContinue
    if ($t) { $ready = $true; break }
}
Write-Output ("Service ready: " + $ready)

# start Qt client
$env:PATH = "$client;$qtdir;C:\Windows\System32"
Remove-Item "$client\c_stdout.log","$client\c_stderr.log" -ErrorAction SilentlyContinue
$cli = Start-Process -FilePath "$client\shadow-worker-client.exe" `
    -WorkingDirectory $client `
    -RedirectStandardOutput "$client\c_stdout.log" `
    -RedirectStandardError "$client\c_stderr.log" `
    -PassThru -WindowStyle Hidden

Start-Sleep -Seconds 5

# check client status
Write-Output "=== Client status ==="
if ($cli.HasExited) {
    Write-Output ("Client EXITED code=" + $cli.ExitCode)
} else {
    Write-Output ("Client RUNNING pid=" + $cli.Id)
    Stop-Process -Id $cli.Id -Force
}

Write-Output ""
Write-Output "=== Server log ==="
if (Test-Path "$backend\server.log") { Get-Content "$backend\server.log" }
Write-Output ""
Write-Output "=== Server err ==="
if (Test-Path "$backend\server_err.log") { Get-Content "$backend\server_err.log" }
Write-Output ""
Write-Output "=== Client stdout ==="
if (Test-Path "$client\c_stdout.log") { Get-Content "$client\c_stdout.log" }
Write-Output ""
Write-Output "=== Client stderr ==="
if (Test-Path "$client\c_stderr.log") { Get-Content "$client\c_stderr.log" }

Stop-Process -Id $svc.Id -Force -ErrorAction SilentlyContinue
