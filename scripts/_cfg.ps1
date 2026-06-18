$vcvars = 'C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat'
cmd /c "`"$vcvars`" && set" | ForEach-Object {
    if ($_ -match '^([^=]+)=(.*)') {
        [System.Environment]::SetEnvironmentVariable($matches[1], $matches[2], 'Process')
    }
}
$env:PATH = 'C:\Users\Administrator\code\1-ai\shadow-worker\tools\protoc\bin;C:\Qt\6.11.1\msvc2022_64\bin;C:\Windows\System32;C:\Windows;C:\Qt\Tools\CMake_64\bin;C:\Qt\Tools\Ninja;' + $env:PATH

Set-Location 'C:\Users\Administrator\code\1-ai\shadow-worker\client'
Remove-Item -Recurse -Force build -ErrorAction SilentlyContinue

# 输出 protoc 是否可见
$protocPath = (Get-Command protoc -ErrorAction SilentlyContinue).Source
Write-Output "protoc at: $protocPath"

$out = & cmake -B build -S . -G Ninja -DCMAKE_PREFIX_PATH='C:\Qt\6.11.1\msvc2022_64' 2>&1
$out | Out-File 'C:\Users\Administrator\code\1-ai\shadow-worker\client\cfg_log.txt'
$errs = $out | Select-String -Pattern 'Grpc|protobuf|protoc|WrapProtoc|find_package'
Write-Output "=== 相关行 ==="
$errs | Select-Object -First 15 -ExpandProperty Line
