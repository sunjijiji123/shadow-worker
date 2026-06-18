$ErrorActionPreference = 'Continue'
$wd = 'C:\Users\Administrator\code\1-ai\shadow-worker\client'
$qt = 'C:\Qt\6.11.1\msvc2022_64\bin'
$vcvars = 'C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat'

# Load MSVC env (LIB/INCLUDE 必须靠它)
cmd /c "`"$vcvars`" && set" | ForEach-Object {
    if ($_ -match '^([^=]+)=(.*)') {
        [System.Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'Process')
    }
}
# protoc 必须在 PATH 最前面,否则 WrapProtoc 找不到
$env:PATH = "C:\Users\Administrator\code\1-ai\shadow-worker\tools\protoc\bin;$qt;C:\Windows\System32;C:\Windows;C:\Qt\Tools\CMake_64\bin;C:\Qt\Tools\Ninja;$env:PATH"

Set-Location $wd

# Clean + reconfigure
Write-Output "=== Clean + Configure ==="
Remove-Item -Recurse -Force build -ErrorAction SilentlyContinue
$cfg = & cmake -B build -S . -G Ninja -DCMAKE_PREFIX_PATH="C:\Qt\6.11.1\msvc2022_64" 2>&1
$cfgErr = $cfg | Select-String -Pattern 'Configuring incomplete|CMake Error'
if ($cfgErr) {
    Write-Output "CONFIG FAILED:"
    $cfgErr | Select-Object -First 5 -ExpandProperty Line
    exit 1
}
Write-Output "CONFIG OK"

Write-Output "=== Build ==="
$build = & cmake --build build 2>&1
$buildErr = $build | Select-String -Pattern 'error C|error LNK|fatal error'
if ($buildErr) {
    Write-Output "BUILD FAILED:"
    $buildErr | Select-Object -First 5 -ExpandProperty Line
    exit 1
}
if (-not (Test-Path 'build\shadow-worker-client.exe')) {
    Write-Output "BUILD FAILED: exe not found"
    exit 1
}
Write-Output "BUILD OK"

# Deploy
Set-Location "$wd\build"
& "$qt\windeployqt.exe" --qmldir '..\qml' 'shadow-worker-client.exe' 2>&1 | Select-Object -Last 1
Write-Output "DEPLOY OK"
