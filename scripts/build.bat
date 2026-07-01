@echo off
REM ============================================================
REM Shadow Worker unified build script
REM ============================================================
REM Usage:
REM   scripts\build.bat backend [clean]   Build Go backend (whisper CGO)
REM   scripts\build.bat client [clean]    Build Qt client (Debug)
REM   scripts\build.bat all [clean]       Build backend + client
REM   scripts\build.bat package           Release build + deploy + installer
REM   scripts\build.bat run [clean]       Build all + run backend + client
REM   scripts\build.bat clean             Clean all build artifacts
REM
REM No setlocal/delayedexpansion (avoids "(x86)" bracket parse bug).
REM ============================================================

REM --- path constants ---
set "ROOT=%~dp0.."
set "VCVARS=C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat"
set "QT_PREFIX=D:\Qt\6.11.1\msvc2022_64"
set "QT_TOOLS=D:\Qt\Tools"
set "GO_BIN=D:\software\Go\bin"
set "ISCC=D:\software\Inno Setup 6\iscc.exe"
set "CLIENT_DIR=%ROOT%\client"
set "BACKEND_DIR=%ROOT%\backend"
set "BUILD_DIR=%ROOT%\build"
set "DIST=%ROOT%\dist"

REM --- dispatch ---
set "CMD=%1"
if "%CMD%"=="" set "CMD=all"
set "OPT=%2"

if /i "%CMD%"=="clean"   goto :do_clean
REM 只有 package（正式出包）才 bump 版本号；开发构建（backend/client/all/run）
REM 只读取 VERSION、不写入——避免调试构建吃掉版本序号（曾因反复 build client
REM 把当天序号从 05 直接跳到 07，06 被吞）。
if /i "%CMD%"=="package" goto :bump_version
if /i "%CMD%"=="backend" goto :read_version
if /i "%CMD%"=="client"  goto :read_version
if /i "%CMD%"=="all"     goto :read_version
if /i "%CMD%"=="run"     goto :read_version

echo [error] unknown command: %CMD%
echo usage: scripts\build.bat [backend^|client^|all^|run^|package^|clean] [clean]
exit /b 1

REM ============================================================
REM Auto-generate version: YYYY.MM.DD.NN (NN = daily sequence)
REM Reads existing VERSION, bumps sequence if same day, else resets to 01.
REM
REM Known limitation: VERSION is both a build artifact and git-tracked.
REM git checkout/commit/pull overwrites the working-copy VERSION with the
REM git value, so the sequence "resets" after a git op. Consecutive builds
REM on the same day (without git ops in between) increment reliably.
REM
REM CRLF robustness: echo writes CRLF, so set /p reads OLD_VER with a
REM trailing \r. VERSION is always "YYYY.MM.DD.NN" = 13 visible chars, and
REM \r sits at byte 14, so slicing OLD_DATE=OLD_VER:~0,10 and
REM OLD_SEQ=OLD_VER:~11,2 never touches \r.
REM
REM Date source (pitfall): an earlier version used `wmic os get localdatetime`,
REM but (1) wmic is deprecated on Win11 24H2+, and (2) non-interactive
REM terminals invoked via PowerShell swallow wmic output, leaving LDT empty.
REM %LDT:~0,4% on an empty var evaluates to the literal "~0,4", producing a
REM garbage version like "~0,4.~4,2.~6,2.01" (once shipped as
REM ShadowWorker-~0,4.~4,2.~6,2.01-setup.exe). Switched to PowerShell
REM Get-Date -Format (locale-independent, stable, not swallowed). Costs ~200ms.
REM Fallback to %DATE% (locale-dependent) when PowerShell is unavailable.
REM NOTE: keep all comments ASCII-only. This file is UTF-8 on disk but cmd.exe
REM parses it as the system ANSI codepage (GBK on zh-CN); non-ASCII bytes can
REM decode to characters containing ")" that break if-block paren balancing.
REM ============================================================
REM read_version: 开发构建（backend/client/all/run）只读 VERSION，不 bump。
REM VERSION 缺失时才写一次（今天.01），保证后续构建有版本号可用。
REM ============================================================
:read_version
set "VERSION_FILE=%ROOT%\VERSION"
set "OLD_VER="
if exist "%VERSION_FILE%" set /p OLD_VER=<"%VERSION_FILE%"

if not "%OLD_VER%"=="" (
    REM 已有 VERSION，直接用，绝不递增（开发构建不吃版本号）。
    set "APP_VERSION=%OLD_VER%"
) else (
    REM 缺失：初始化为今天.01 并写一次（首次检出/被 clean 掉的情况）。
    call :today_date
    set "APP_VERSION=%TODAY%.01"
    echo %APP_VERSION%> "%VERSION_FILE%"
    echo [version] %APP_VERSION% ^(initialized VERSION — dev build does not bump^)
    goto :setup_env
)
echo [version] %APP_VERSION%
goto :setup_env

REM ============================================================
REM bump_version: 仅 package（正式出包）调用。读旧版本、bump 序号、写回。
REM ============================================================
:bump_version
set "VERSION_FILE=%ROOT%\VERSION"

call :today_date

REM Read existing version (set /p reads one line; trailing \r from CRLF).
set "OLD_VER="
if exist "%VERSION_FILE%" set /p OLD_VER=<"%VERSION_FILE%"

REM Parse old date (first 10 chars YYYY.MM.DD) and sequence (chars 12-13 NN).
REM Strict slicing; \r at byte 14 is never included.
set "OLD_DATE="
set "OLD_SEQ=00"
if not "%OLD_VER%"=="" (
    set "OLD_DATE=%OLD_VER:~0,10%"
    set "OLD_SEQ=%OLD_VER:~11,2%"
)

REM Determine new sequence and write VERSION.
REM Need enabledelayedexpansion for arithmetic + zero-pad.
setlocal enabledelayedexpansion
REM Abort if TODAY is still empty (PowerShell and %DATE% both failed) to
REM avoid writing a garbage version.
if "!TODAY!"=="" (
    echo [ERROR] Cannot determine current date ^(PowerShell and %%DATE%% both failed^), aborting.
    endlocal
    exit /b 1
)
set "NEW_SEQ=01"
if "!OLD_DATE!"=="!TODAY!" (
    set /a "NEW_SEQ=!OLD_SEQ!+1"
    if !NEW_SEQ! LSS 10 set "NEW_SEQ=0!NEW_SEQ!"
)
set "NEW_VER=!TODAY!.!NEW_SEQ!"
REM echo writes CRLF (trailing \r\n in VERSION). Reader side handles \r.
echo !NEW_VER!> "%VERSION_FILE%"
REM Export values past endlocal.
for /f "tokens=*" %%v in ("!NEW_VER!") do (
    endlocal & set "APP_VERSION=%%v"
)
echo [version] %APP_VERSION% ^(written to VERSION^)
goto :setup_env

REM ============================================================
REM today_date: 设 TODAY=yyyy.MM.dd（PowerShell 优先，%DATE% 兜底）。
REM ============================================================
:today_date
set "TODAY="
for /f "delims=" %%d in ('powershell -NoProfile -Command "Get-Date -Format yyyy.MM.dd" 2^>nul') do set "TODAY=%%d"
if "%TODAY%"=="" (
    REM Fallback: %DATE% is locale-dependent; only a last resort.
    set "TODAY=%DATE:/=.%"
)
goto :eof

REM ============================================================
REM Environment setup (shared by all build commands except clean)
REM ============================================================
:setup_env
echo [env] loading vcvars64 + Qt + Go...
call "%VCVARS%" >nul 2>&1
if errorlevel 1 (
    echo [FAIL] vcvars64 failed
    exit /b 1
)
set "PATH=%QT_TOOLS%\CMake_64\bin;%QT_TOOLS%\Ninja;%GO_BIN%;%QT_PREFIX%\bin;%PATH%"
where cl >nul 2>&1 || (echo [FAIL] cl.exe missing & exit /b 1)
where ninja >nul 2>&1 || (echo [FAIL] ninja missing & exit /b 1)
echo [env] OK

if /i "%CMD%"=="backend" goto :do_backend
if /i "%CMD%"=="client"  goto :do_client
if /i "%CMD%"=="all"     goto :do_backend
if /i "%CMD%"=="run"     goto :do_backend
if /i "%CMD%"=="package" goto :do_pkg_backend

REM ============================================================
REM Backend build (delegates to build-whisper-cgo.bat)
REM ============================================================
:do_backend
echo.
echo === Backend build ===
cd /d "%BACKEND_DIR%"
call "%BACKEND_DIR%\build-whisper-cgo.bat" "%BUILD_DIR%\shadow-worker.exe"
if errorlevel 1 (
    echo [FAIL] backend build failed
    exit /b 1
)
echo [OK] backend: %BUILD_DIR%\shadow-worker.exe
copy /Y "%VERSION_FILE%" "%BUILD_DIR%\VERSION" >nul

if /i "%CMD%"=="backend" goto :eof_ok
if /i "%CMD%"=="all"     goto :do_client
if /i "%CMD%"=="run"     goto :do_client

REM ============================================================
REM Client build (cmake Debug)
REM ============================================================
:do_client
echo.
echo === Client build ===
cd /d "%CLIENT_DIR%"

if /i "%OPT%"=="clean" (
    echo [clean] removing client\build
    rmdir /s /q build 2>nul
)
if /i "%CMD%"=="run" if /i "%OPT%"=="clean" (
    echo [clean] removing client\build
    rmdir /s /q build 2>nul
)

if not exist build\CMakeCache.txt (
    echo [configure] cmake -B build
    cmake -B build -S . -G Ninja -DCMAKE_PREFIX_PATH="%QT_PREFIX%" -DCMAKE_BUILD_TYPE=Debug
    if errorlevel 1 (
        echo [FAIL] cmake configure failed
        exit /b 1
    )
)

echo [build] cmake --build build --config Debug
cmake --build build --config Debug
if errorlevel 1 (
    echo [FAIL] client build failed
    exit /b 1
)
echo [OK] client: %CLIENT_DIR%\build\shadow-worker-client.exe
copy /Y "%VERSION_FILE%" "%CLIENT_DIR%\build\VERSION" >nul

if /i "%CMD%"=="all" goto :eof_ok
if /i "%CMD%"=="run" goto :do_run

REM ============================================================
REM Run backend + client
REM ============================================================
:do_run
echo.
echo === Run ===
echo [run] starting backend...
start "" /D "%BUILD_DIR%" "%BUILD_DIR%\shadow-worker.exe"
echo [run] starting client (Qt DLLs from Qt bin)...
set "PATH=%QT_PREFIX%\bin;%PATH%"
start "" /D "%CLIENT_DIR%\build" "%CLIENT_DIR%\build\shadow-worker-client.exe"
echo [OK] both started
goto :eof_ok

REM ============================================================
REM Package: release build + deploy + installer
REM ============================================================
:do_pkg_backend
echo.
echo === Package: clean dist\bin (keep old installers) ===
if exist "%DIST%\bin" rmdir /S /Q "%DIST%\bin"
mkdir "%DIST%\bin"

echo.
echo === Package: backend (release) ===
cd /d "%BACKEND_DIR%"
call "%BACKEND_DIR%\build-whisper-cgo.bat" "%DIST%\bin\shadow-worker.exe"
if errorlevel 1 (
    echo [FAIL] backend CGO build failed
    exit /b 1
)

echo.
echo === Package: mcp standalone exe (pure Go, no CGO) ===
REM MCP server 只依赖 storage(modernc.org/sqlite 纯 Go)+ MCP SDK，不碰 whisper/CGO，
REM 故用普通 go build（无需 gcc，几秒完成）。拆成独立 exe 是为了让 agent 持有的
REM MCP 子进程跑在独立文件上，升级主程序时覆盖 shadow-worker.exe 不再被锁文件阻断
REM （AGENTS.md 坑 50）。
cd /d "%BACKEND_DIR%"
go build -o "%DIST%\bin\shadow-worker-mcp.exe" ./cmd/shadow-worker-mcp/
if errorlevel 1 (
    echo [FAIL] mcp standalone build failed
    exit /b 1
)

echo.
echo === Package: client (release) ===
cd /d "%CLIENT_DIR%"
rmdir /s /q build 2>nul
cmake -B build -S . -G Ninja -DCMAKE_PREFIX_PATH="%QT_PREFIX%" -DCMAKE_BUILD_TYPE=Release
if errorlevel 1 goto :cmake_configure_fail
cmake --build build
if errorlevel 1 goto :client_build_fail
copy /Y "%CLIENT_DIR%\build\shadow-worker-client.exe" "%DIST%\bin\" >nul
copy /Y "%CLIENT_DIR%\build\*_grpc.dll" "%DIST%\bin\" >nul
copy /Y "%CLIENT_DIR%\build\*_proto.dll" "%DIST%\bin\" >nul
copy /Y "%VERSION_FILE%" "%DIST%\bin\VERSION" >nul

echo.
echo === Package: windeployqt ===
windeployqt "%DIST%\bin\shadow-worker-client.exe" --release --qmldir "%CLIENT_DIR%\qml" --no-translations --no-compiler-runtime
if errorlevel 1 (
    echo [FAIL] windeployqt failed
    exit /b 1
)

echo.
echo === Package: VC runtime ===
if defined VCToolsRedistDir (
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\msvcp140.dll" "%DIST%\bin\" >nul
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\vcruntime140.dll" "%DIST%\bin\" >nul
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\vcruntime140_1.dll" "%DIST%\bin\" >nul
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\concrt140.dll" "%DIST%\bin\" >nul
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\msvcp140_1.dll" "%DIST%\bin\" >nul
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\msvcp140_2.dll" "%DIST%\bin\" >nul
    echo [OK] VC runtime copied
) else (
    echo [warn] VCToolsRedistDir not set, VC runtime may be missing
)

echo.
echo === Package: Inno Setup installer ===
if not exist "%ISCC%" (
    echo [warn] iscc.exe not found at %ISCC%
    echo [warn] portable files ready: %DIST%\bin
    goto :eof_ok
)
"%ISCC%" /DAPP_VERSION="%APP_VERSION%" "%ROOT%\package\ShadowWorker.iss"
if errorlevel 1 (
    echo [FAIL] Inno Setup compile failed
    exit /b 1
)
echo.
echo === Package done ===
echo Installer: %DIST%\ShadowWorker-%APP_VERSION%-setup.exe
goto :eof_ok

REM ============================================================
REM Clean all build artifacts
REM ============================================================
:do_clean
echo === Clean all build artifacts ===
if exist "%CLIENT_DIR%\build" (
    echo [clean] client\build
    rmdir /s /q "%CLIENT_DIR%\build"
)
if exist "%BUILD_DIR%" (
    echo [clean] build (backend)
    rmdir /s /q "%BUILD_DIR%"
)
if exist "%DIST%" (
    echo [clean] dist
    rmdir /s /q "%DIST%"
)
echo [OK] clean done
goto :eof_ok

REM ============================================================
:client_build_fail
echo [FAIL] client build failed
exit /b 1

:cmake_configure_fail
echo [FAIL] cmake configure failed
exit /b 1

:eof_ok
echo.
echo [done] %CMD% completed successfully
