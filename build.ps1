# build.ps1 - Shadow Worker 统一构建脚本（PowerShell）。
#
# 为什么是 PowerShell 而不是 .bat：
#   在 AI agent / 某些非交互终端里，cmd.exe 的 stdout 会被 VT 清屏序列吞掉
#   （banner 显示后立即 [H[2J[3J 返回，echo/set/go build 的输出和 `> log`
#   重定向全部失效，但 exit code 仍正常）。PowerShell 不受此影响，输出完整。
#   故本脚本用 PowerShell 编排，仅在确需 vcvars64 的子步骤调 .bat。
#
# 用法:
#   .\build.ps1                          # 等价于 build all
#   .\build.ps1 build                    # 构建客户端 + 后端
#   .\build.ps1 build client             # 仅客户端（vcvars64 + cmake）
#   .\build.ps1 build server             # 仅后端（CGO + whisper.cpp）
#   .\build.ps1 build proto              # 仅重新生成 Go proto stub
#   .\build.ps1 clean                    # 清理 client/build + backend 构建缓存
#
# 覆盖的坑（AGENTS.md）:
#   #2  MSVC type_traits 需 vcvars64 -> build_client.bat 已处理
#   #9  gcc 16 不兼容 cgo -> build-whisper-cgo.bat 锁定 w64devkit gcc 13.2.0
#   #10 vendor patch -lgomp -> build-whisper-cgo.bat 每次构建前自动 patch
#   #20 改 QML 后 ninja "no work to do" -> 本脚本 -CleanQml 时删 .qt\rcc 强制重打 qrc
#   #26/#35 gen_proto.bat 只处理 overview -> 本脚本 proto 模式遍历全部 .proto
#   #27 LNK1168 exe 被锁 -> 构建前 Stop-Process shadow-worker-client.exe
#
# 环境变量（可覆盖默认探测路径）:
#   $env:VCVARS_PATH   vcvars64.bat 全路径
#   $env:QT_PREFIX     Qt msvc 前缀（如 D:\Qt\6.11.1\msvc2022_64）
#   $env:QT_TOOLS      Qt Tools 目录（含 CMake_64/Ninja）
#   $env:W64DEVKIT     w64devkit 根（gcc 13.2.0，后端 CGO 用）
#   $env:GO_BIN        Go bin 目录（含 go.exe + protoc.exe）
#   $env:PROTOC        protoc.exe 全路径（默认在 GO_BIN 下找）

param(
    [ValidateSet("build", "clean")]
    [string]$Action = "build",

    [ValidateSet("client", "server", "proto", "all")]
    [string]$Target = "all",

    # 改了 .qml 后强制重新打包 qrc（坑 #20）。
    [switch]$CleanQml,

    # 跳过环境检查（已确认环境 OK 时加速重复构建）。
    [switch]$SkipEnvCheck
)
$ErrorActionPreference = "Stop"
$proj = $PSScriptRoot

# ============================================================
# 默认路径探测（可被同名环境变量覆盖）
# ============================================================
function Get-EnvOrDefault($name, $default) {
    # 动态访问环境变量: $env:NAME 是静态语法，动态名要用 Get-Item -LiteralPath "Env:NAME"。
    $val = (Get-Item -LiteralPath "Env:$name" -ErrorAction SilentlyContinue).Value
    if ($val -and (Test-Path $val)) { return $val }
    return $default
}

$Paths = @{
    VCVARS    = Get-EnvOrDefault "VCVARS_PATH" "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat"
    QT_PREFIX = Get-EnvOrDefault "QT_PREFIX"  "D:\Qt\6.11.1\msvc2022_64"
    QT_TOOLS  = Get-EnvOrDefault "QT_TOOLS"   "D:\Qt\Tools"
    W64DEVKIT = Get-EnvOrDefault "W64DEVKIT"  "D:\Qt\w64devkit"
    GO_BIN    = Get-EnvOrDefault "GO_BIN"     "D:\software\Go\bin"
}

# protoc 默认在 GO_BIN 下，否则读 $env:PROTOC。
$protoc = if ($env:PROTOC -and (Test-Path $env:PROTOC)) { $env:PROTOC }
          else { Join-Path $Paths.GO_BIN "protoc.exe" }

$dirs = @{
    Client = Join-Path $proj "client"
    Backend = Join-Path $proj "backend"
    Proto   = Join-Path $proj "proto"
    Build   = Join-Path $proj "build"
}

# ============================================================
# 工具函数
# ============================================================

function Test-CommandExists($name) {
    $null -ne (Get-Command $name -ErrorAction SilentlyContinue)
}

function Stop-ClientProcess {
    # 坑 #27: 链接阶段 LNK1168 几乎都是 client.exe 在运行锁住 DLL。
    # 构建前主动杀，避免链接失败。后端是控制台程序，不锁客户端 DLL。
    $procs = Get-Process shadow-worker-client -ErrorAction SilentlyContinue
    if ($procs) {
        Write-Host "  killing running shadow-worker-client.exe" -ForegroundColor Yellow
        $procs | Stop-Process -Force
        Start-Sleep -Milliseconds 600  # 等待文件句柄释放
    }
}

function Clear-QmlCache {
    # 坑 #20: 改了 .qml 后 ninja 有时判定 "no work to do"，运行的是旧 QML。
    # 删 client\build\.qt\rcc 强制 qt_add_qml_module 重新打包 qrc。
    # 注意：删 rcc 后必须连带删 CMakeCache.txt，否则 cmake configure 被
    # 跳过，build 阶段会因 .qrc 文件缺失而报 "missing and no known rule to make it"。
    $rcc = Join-Path $dirs.Client "build\.qt\rcc"
    if (Test-Path $rcc) {
        Write-Host "  clearing QML rcc cache (坑 #20)" -ForegroundColor Yellow
        Remove-Item -Recurse -Force $rcc
    }
    $cache = Join-Path $dirs.Client "build\CMakeCache.txt"
    if (Test-Path $cache) {
        Remove-Item -Force $cache
    }
}

function Clear-QmlShadow {
    # 坑 #20 补充: build\ShadowWorker 下有一份复制的 QML，也要删。
    $shadow = Join-Path $dirs.Client "build\ShadowWorker"
    if (Test-Path $shadow) {
        Remove-Item -Recurse -Force $shadow
    }
}

# ============================================================
# Check-Env: 前置环境检查，缺工具就列出错误并退出。
# ============================================================
function Check-Env {
    Write-Host "[build.ps1] Checking environment ..." -ForegroundColor Cyan
    $errors = @()

    # Go + protoc（后端 CGO + proto 生成都要）
    if (Test-CommandExists "go") {
        $gv = (go version 2>$null) -replace 'go version go(\S+).*', '$1'
        Write-Host "  go       $gv"
    } else {
        # go 不在 PATH 时，尝试用 GO_BIN 下的
        $goExe = Join-Path $Paths.GO_BIN "go.exe"
        if (Test-Path $goExe) {
            $env:PATH = "$($Paths.GO_BIN);$env:PATH"
            Write-Host "  go       added $($Paths.GO_BIN) to PATH"
        } else {
            $errors += "go not found. Set `$env:GO_BIN (e.g. D:\software\Go\bin)."
        }
    }

    # protoc
    if (Test-Path $protoc) {
        Write-Host "  protoc   $protoc"
        $protocDir = Split-Path $protoc -Parent
        if ($env:PATH -notlike "*$protocDir*") {
            $env:PATH = "$protocDir;$env:PATH"
        }
    } else {
        $errors += "protoc not found at $protoc. Set `$env:PROTOC."
    }

    # MSVC 工具链（client 构建必需）。
    # 不依赖 vcvars64.bat：本脚本用 Set-MsvcEnv 直接设 INCLUDE/LIB/PATH，
    # 规避 cmd 嵌套调 vcvars64 的"此时不应有 \Microsoft"括号解析 bug。
    # 这里只校验 MSVC tools 目录存在（与 Set-MsvcEnv 的探测路径一致）。
    $needMsvc = ($Target -eq "all" -or $Target -eq "client")
    if ($needMsvc) {
        $vsRoot = "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC"
        if (Test-Path $vsRoot) {
            $msvcVer = (Get-ChildItem -Path $vsRoot -Directory | Sort-Object Name -Descending | Select-Object -First 1).Name
            Write-Host "  MSVC     $msvcVer (env will be set without vcvars64)"
        } else {
            $errors += "MSVC tools dir not found at $vsRoot."
        }
        # Qt prefix
        if (Test-Path $Paths.QT_PREFIX) {
            Write-Host "  Qt       $($Paths.QT_PREFIX)"
        } else {
            $errors += "Qt prefix not found at $($Paths.QT_PREFIX). Set `$env:QT_PREFIX."
        }
        # CMake + Ninja（Qt Tools 下）
        $cmake = Join-Path $Paths.QT_TOOLS "CMake_64\bin\cmake.exe"
        $ninja = Join-Path $Paths.QT_TOOLS "Ninja\ninja.exe"
        if ((Test-Path $cmake) -and (Test-Path $ninja)) {
            Write-Host "  cmake    $cmake"
            Write-Host "  ninja    $ninja"
        } else {
            $errors += "CMake/Ninja not found under $($Paths.QT_TOOLS). Set `$env:QT_TOOLS."
        }
    }

    # w64devkit gcc（server CGO 必需，坑 #9: 必须 gcc 13.2.0，gcc 16 不兼容）
    $needGcc = ($Target -eq "all" -or $Target -eq "server")
    if ($needGcc) {
        $gcc = Join-Path $Paths.W64DEVKIT "bin\gcc.exe"
        if (Test-Path $gcc) {
            # 检查版本，gcc 16 会警告
            $gccVer = & $gcc -dumpversion 2>$null
            Write-Host "  gcc      $gccVer ($gcc)"
            if ($gccVer -and [int]($gccVer -split '\.')[0] -ge 14) {
                Write-Host "  WARNING  gcc $gccVer may be incompatible with Go cgo (坑 #9: use gcc 13.x)" -ForegroundColor Red
            }
        } else {
            $errors += "w64devkit gcc not found at $gcc. Set `$env:W64DEVKIT (坑 #9: 需要 gcc 13.2.0)."
        }
    }

    if ($errors.Count -gt 0) {
        Write-Host "`n[build.ps1] Environment errors:" -ForegroundColor Red
        foreach ($e in $errors) { Write-Host "  - $e" -ForegroundColor Red }
        exit 1
    }
    Write-Host ""
}

# ============================================================
# 构建步骤
# ============================================================

function Set-MsvcEnv {
    # 不调 vcvars64.bat：它在 cmd 嵌套（PowerShell→cmd→bat）下会触发
    # "此时不应有 \Microsoft" 的路径括号解析 bug，且 PowerShell 加载含
    # "写临时 bat + cmd /c 执行" 模式的脚本会被 AMSI 判可疑而崩溃。
    #
    # vcvars64 的本质只是设 INCLUDE/LIB/PATH 三组环境变量，我们直接设等价值，
    # 与调用 vcvars64 完全等价（已实测：这样 cmake --build 能找到 type_traits，
    # categoryproxy 等全部编译通过）。MSVC 版本目录自动探测（不硬编码）。
    $vsRoot = "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Tools\MSVC"
    $msvcDir = (Get-ChildItem -Path $vsRoot -Directory | Sort-Object Name -Descending | Select-Object -First 1).FullName
    if (-not $msvcDir) { throw "MSVC tools dir not found under $vsRoot" }

    $sdkRoot = "C:\Program Files (x86)\Windows Kits\10"
    $sdkVer = (Get-ChildItem -Path (Join-Path $sdkRoot "Include") -Directory |
               Where-Object { $_.Name -match '^\d+\.' } |
               Sort-Object Name -Descending | Select-Object -First 1).Name
    if (-not $sdkVer) { throw "Windows SDK not found under $sdkRoot" }

    # INCLUDE: MSVC 头 + UCRT + um + shared（顺序即 vcvars64 的顺序）
    $env:INCLUDE = ($msvcDir + "\include",
                    "$sdkRoot\Include\$sdkVer\ucrt",
                    "$sdkRoot\Include\$sdkVer\um",
                    "$sdkRoot\Include\$sdkVer\shared") -join ';'
    # LIB: MSVC lib/x64 + UCRT lib/x64 + um lib/x64
    $env:LIB = ($msvcDir + "\lib\x64",
                "$sdkRoot\Lib\$sdkVer\ucrt\x64",
                "$sdkRoot\Lib\$sdkVer\um\x64") -join ';'
    # PATH: cl.exe/link.exe 在 Hostx64/x64
    $env:PATH = "$msvcDir\bin\Hostx64\x64;$env:PATH"

    Write-Host "  MSVC     $(Split-Path $msvcDir -Leaf) + SDK $sdkVer (env set without vcvars64)" -ForegroundColor DarkGray
}

function Invoke-BuildClient {
    Write-Host "[build.ps1] Building Qt client ..." -ForegroundColor Cyan

    # 坑 #27: 先杀可能运行的客户端（LNK1168 exe 被锁）
    Stop-ClientProcess
    # 坑 #20: 改 QML 后强制重打 qrc
    if ($CleanQml) { Clear-QmlCache; Clear-QmlShadow }

    # 设 MSVC 环境（替代 vcvars64，规避 cmd 嵌套坑 + AMSI 崩溃）
    Set-MsvcEnv

    $qtPrefix = $Paths.QT_PREFIX
    $cmake = Join-Path $Paths.QT_TOOLS "CMake_64\bin\cmake.exe"
    Push-Location $dirs.Client
    try {
        # configure（无 cache 时）
        $cache = Join-Path $dirs.Client "build\CMakeCache.txt"
        if (-not (Test-Path $cache)) {
            Write-Host "  cmake configure ..."
            & $cmake -B build -S . -G Ninja -DCMAKE_PREFIX_PATH="$qtPrefix" -DCMAKE_BUILD_TYPE=Debug
            if ($LASTEXITCODE -ne 0) { throw "cmake configure failed" }
        }
        # build
        Write-Host "  cmake --build ..."
        & $cmake --build build --config Debug
        if ($LASTEXITCODE -ne 0) { throw "cmake build failed" }
    } finally { Pop-Location }

    $exe = Join-Path $dirs.Client "build\shadow-worker-client.exe"
    Write-Host "[build.ps1] Client OK: $exe" -ForegroundColor Green
}

function Invoke-BuildServer {
    Write-Host "[build.ps1] Building Go backend (CGO + whisper.cpp) ..." -ForegroundColor Cyan

    $wsBat = Join-Path $dirs.Backend "build-whisper-cgo.bat"
    if (-not (Test-Path $wsBat)) {
        throw "backend\build-whisper-cgo.bat not found"
    }

    # build-whisper-cgo.bat 内部已处理: vendor patch -lgomp (坑 #10) +
    # w64devkit gcc 13.2.0 (坑 #9) + CGO 工具链环境。
    & cmd /c "`"$wsBat`""
    if ($LASTEXITCODE -ne 0) { throw "Go backend build failed (exit $LASTEXITCODE)" }

    $exe = Join-Path $dirs.Build "shadow-worker.exe"
    Write-Host "[build.ps1] Server OK: $exe" -ForegroundColor Green
}

function Invoke-GenProto {
    # 坑 #26/#35: gen_proto.bat 只处理 overview.proto，
    # 其他 5 个（asr/collection/config/voice/whitelist）要手动 protoc。
    # 本函数遍历 proto\ 下所有 .proto，逐一重新生成 Go stub。
    # Qt stub 由 cmake --build 的 qt_add_protobuf 自动重生，无需此处处理。
    Write-Host "[build.ps1] Generating Go proto stubs ..." -ForegroundColor Cyan

    if (-not (Test-Path $protoc)) { throw "protoc not found: $protoc" }

    $goOut = Join-Path $dirs.Backend "internal\grpcapi"
    if (-not (Test-Path $goOut)) { New-Item -ItemType Directory -Force -Path $goOut | Out-Null }

    $protos = Get-ChildItem -Path $dirs.Proto -Filter "*.proto"
    if ($protos.Count -eq 0) { throw "No .proto found in $($dirs.Proto)" }

    foreach ($p in $protos) {
        Write-Host "  $($p.Name)"
        & $protoc -I $dirs.Proto `
            --go_out=$goOut --go_opt=paths=source_relative `
            --go-grpc_out=$goOut --go-grpc_opt=paths=source_relative `
            $p.FullName
        if ($LASTEXITCODE -ne 0) { throw "protoc failed on $($p.Name)" }
    }
    Write-Host "[build.ps1] Proto OK: Go stubs in $goOut" -ForegroundColor Green
    Write-Host "  (Qt stubs 由 cmake --build 的 qt_add_protobuf 自动重生)" -ForegroundColor DarkGray
}

function Invoke-Clean {
    Write-Host "[build.ps1] Cleaning build artifacts ..." -ForegroundColor Cyan
    # 客户端
    $clientBuild = Join-Path $dirs.Client "build"
    if (Test-Path $clientBuild) {
        Write-Host "  removing $clientBuild"
        Remove-Item -Recurse -Force $clientBuild
    }
    # 后端产物（保留 vendor / third_party）
    foreach ($f in @("shadow-worker.exe")) {
        $p = Join-Path $dirs.Build $f
        if (Test-Path $p) { Write-Host "  removing $p"; Remove-Item -Force $p }
    }
    Write-Host "[build.ps1] Clean done. Re-run build to regenerate." -ForegroundColor Green
}

# ============================================================
# Main
# ============================================================

if (-not $SkipEnvCheck) { Check-Env }

# 把 Go bin 前置到 PATH（go/protoc 命令解析依赖）
$goBin = $Paths.GO_BIN
if ((Test-Path $goBin) -and ($env:PATH -notlike "*$goBin*")) {
    $env:PATH = "$goBin;$env:PATH"
}

switch ($Action) {
    "clean" { Invoke-Clean; exit 0 }
    "build" {
        switch ($Target) {
            "proto"  { Invoke-GenProto }
            "client" { Invoke-BuildClient }
            "server" { Invoke-BuildServer }
            "all"    { Invoke-GenProto; Invoke-BuildServer; Invoke-BuildClient }
        }
        Write-Host "`n[build.ps1] Build complete ($Target)." -ForegroundColor Green
    }
}
