$ErrorActionPreference = 'Continue'
$wd = 'C:\Users\Administrator\code\1-ai\shadow-worker\client'
$qt = 'C:\Qt\6.11.1\msvc2022_64\bin'
$vcvars = 'C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat'

# 关键:先调 vcvars64 设好 MSVC 环境(LIB/INCLUDE),再编译
# 用 cmd 包一层,让 vcvars64 的环境变量在当前 PowerShell 会话生效
Write-Output "=== 加载 MSVC 环境 ==="
cmd /c "`"$vcvars`" && set" | ForEach-Object {
    if ($_ -match '^([^=]+)=(.*)') {
        [System.Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'Process')
    }
}
$env:PATH = "$qt;C:\Windows\System32;C:\Windows;C:\Qt\Tools\CMake_64\bin;C:\Qt\Tools\Ninja;$env:PATH"
Write-Output "cl.exe: $((Get-Command cl.exe -ErrorAction SilentlyContinue).Source)"

Set-Location $wd
Write-Output "=== 编译 ==="
& cmake --build build 2>&1 | Select-Object -Last 3

if (-not (Test-Path 'build\shadow-worker-client.exe')) {
    Write-Output "BUILD_FAILED"
    exit 1
}

Write-Output "=== 部署 ==="
Set-Location "$wd\build"
& "$qt\windeployqt.exe" --qmldir '..\qml' 'shadow-worker-client.exe' 2>&1 | Select-Object -Last 2

Write-Output "=== 运行(8s 超时) ==="
$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = "$wd\build\shadow-worker-client.exe"
$psi.WorkingDirectory = "$wd\build"
$psi.UseShellExecute = $false
$psi.RedirectStandardError = $true
$psi.RedirectStandardOutput = $true
$p = [System.Diagnostics.Process]::Start($psi)
$p.WaitForExit(8000) | Out-Null
Write-Output "EXIT_CODE=$($p.ExitCode)"
Write-Output "--- stderr ---"
$p.StandardError.ReadToEnd()
