$exe = 'C:\Users\Administrator\code\1-ai\shadow-worker\client\build\shadow-worker-client.exe'
$wd  = 'C:\Users\Administrator\code\1-ai\shadow-worker\client\build'
$env:PATH = "$wd;C:\Windows\System32;C:\Windows"
# 启用 QML 警告输出到 stderr
$env:QT_LOGGING_RULES = 'qt.qml.binding.removal=true;qml=true'
$env:QT_DEBUG_PLUGINS = '0'
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $exe
$psi.WorkingDirectory = $wd
$psi.UseShellExecute = $false
$psi.RedirectStandardError = $true
$psi.RedirectStandardOutput = $true
$p = [System.Diagnostics.Process]::Start($psi)
$p.WaitForExit(8000) | Out-Null
Write-Output "EXIT_CODE=$($p.ExitCode)"
Write-Output "--- stderr ---"
Write-Output $p.StandardError.ReadToEnd()
