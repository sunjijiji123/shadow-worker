$ErrorActionPreference = 'Continue'
$wd = 'C:\Users\Administrator\code\1-ai\shadow-worker\client'
$qt = 'C:\Qt\6.11.1\msvc2022_64\bin'
$vcvars = 'C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat'

cmd /c "`"$vcvars`" && set" | ForEach-Object {
    if ($_ -match '^([^=]+)=(.*)') {
        [System.Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'Process')
    }
}
$env:PATH = "C:\Users\Administrator\code\1-ai\shadow-worker\tools\protoc\bin;$qt;C:\Windows\System32;C:\Windows;C:\Qt\Tools\CMake_64\bin;C:\Qt\Tools\Ninja;$env:PATH"

Set-Location $wd
Write-Output "=== Configure (incremental) ==="
$cfg = & cmake -B build -S . -G Ninja -DCMAKE_PREFIX_PATH="C:\Qt\6.11.1\msvc2022_64" 2>&1
$cfgErr = $cfg | Select-String -Pattern 'Configuring incomplete|CMake Error'
if ($cfgErr) {
    Write-Output "CONFIG FAILED:"
    $cfgErr | Select-Object -First 8 -ExpandProperty Line
    exit 1
}
Write-Output "CONFIG OK"

Write-Output "=== Build ==="
$build = & cmake --build build 2>&1
$buildErr = $build | Select-String -Pattern 'error C|error LNK|fatal error'
if ($buildErr) {
    Write-Output "BUILD FAILED:"
    $buildErr | Select-Object -First 15 -ExpandProperty Line
    exit 1
}
Write-Output "BUILD OK"
