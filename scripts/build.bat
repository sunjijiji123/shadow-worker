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
if /i "%CMD%"=="backend" goto :gen_version
if /i "%CMD%"=="client"  goto :gen_version
if /i "%CMD%"=="all"     goto :gen_version
if /i "%CMD%"=="run"     goto :gen_version
if /i "%CMD%"=="package" goto :gen_version

echo [error] unknown command: %CMD%
echo usage: scripts\build.bat [backend^|client^|all^|run^|package^|clean] [clean]
exit /b 1

REM ============================================================
REM Auto-generate version: YYYY.MM.DD.NN (NN = daily sequence)
REM Reads existing VERSION, bumps sequence if same day, else resets to 01.
REM ============================================================
:gen_version
set "VERSION_FILE=%ROOT%\VERSION"
REM Get today's date parts via wmic (locale-independent)
for /f "tokens=2 delims==" %%a in ('wmic os get localdatetime /value 2^>nul ^| find "="') do set "LDT=%%a"
set "TODAY_Y=%LDT:~0,4%"
set "TODAY_M=%LDT:~4,2%"
set "TODAY_D=%LDT:~6,2%"
set "TODAY=%TODAY_Y%.%TODAY_M%.%TODAY_D%"

REM Read existing version
set "OLD_VER="
if exist "%VERSION_FILE%" set /p OLD_VER=<"%VERSION_FILE%"

REM Parse old date part (first 10 chars: YYYY.MM.DD) and sequence (last 2)
set "OLD_DATE="
set "OLD_SEQ=00"
if not "%OLD_VER%"=="" (
    set "OLD_DATE=%OLD_VER:~0,10%"
    set "OLD_SEQ=%OLD_VER:~11,2%"
)

REM Determine new sequence and write VERSION
REM Need enabledelayedexpansion for the arithmetic + zero-pad
setlocal enabledelayedexpansion
set "NEW_SEQ=01"
if "!OLD_DATE!"=="%TODAY%" (
    set /a "NEW_SEQ=!OLD_SEQ!+1"
    if !NEW_SEQ! LSS 10 set "NEW_SEQ=0!NEW_SEQ!"
)
set "NEW_VER=%TODAY%.!NEW_SEQ!"
echo !NEW_VER!> "%VERSION_FILE%"
REM Export values past endlocal
for /f "tokens=*" %%v in ("!NEW_VER!") do (
    endlocal & set "APP_VERSION=%%v"
)
echo [version] %APP_VERSION% ^(written to VERSION^)
goto :setup_env

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
