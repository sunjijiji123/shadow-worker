$env:PATH = 'C:\Users\Administrator\code\1-ai\shadow-worker\client\build;C:\Windows\System32;C:\Windows'
Set-Location 'C:\Users\Administrator\code\1-ai\shadow-worker\client\build'
Start-Process -FilePath '.\shadow-worker-client.exe' -WorkingDirectory $PWD
Start-Sleep -Seconds 3
$p = Get-Process shadow-worker-client -ErrorAction SilentlyContinue
if ($p) {
    Write-Output "PROCESS_OK pid=$($p.Id)"
} else {
    Write-Output "PROCESS_NOT_FOUND"
}
