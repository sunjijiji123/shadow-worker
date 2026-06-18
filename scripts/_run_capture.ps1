$wd = 'C:\Users\Administrator\code\1-ai\shadow-worker\client\build'
$qt = 'C:\Qt\6.11.1\msvc2022_64\bin'
$env:PATH = "$wd;$qt;C:\Windows\System32;C:\Windows"

# 删旧日志
Remove-Item "$wd\run_stdout.log","$wd\run_stderr.log" -ErrorAction SilentlyContinue

# Start-Process 重定向到文件,然后读文件
$proc = Start-Process -FilePath "$wd\shadow-worker-client.exe" `
    -WorkingDirectory $wd `
    -RedirectStandardOutput "$wd\run_stdout.log" `
    -RedirectStandardError "$wd\run_stderr.log" `
    -PassThru

Start-Sleep -Seconds 5
if (-not $proc.HasExited) {
    Write-Output "进程还活着,杀掉"
    $proc | Stop-Process -Force
    Write-Output "EXIT_CODE=killed"
} else {
    Write-Output "EXIT_CODE=$($proc.ExitCode)"
}

Write-Output "--- stdout (前 50 行) ---"
if (Test-Path "$wd\run_stdout.log") { Get-Content "$wd\run_stdout.log" -Head 50 }
Write-Output "--- stderr (前 50 行) ---"
if (Test-Path "$wd\run_stderr.log") { Get-Content "$wd\run_stderr.log" -Head 50 }
