@echo off
REM ============================================================
REM Shadow Worker dev environment loader
REM ============================================================
REM Usage: run this in a dev shell before building.
REM   D:\code\shadow-worker\scripts\setenv.bat
REM After that, go build / cmake --build work directly.
REM
REM Effects:
REM   1. Clean MSYS2/MinGW from PATH (avoid polluting MSVC link)
REM   2. Load MSVC environment (vcvars64, sets LIB/INCLUDE)
REM   3. Put Qt6 MSVC bin / CMake / Ninja / Go bin / protoc at PATH front
REM   4. Fix Go env vars polluted on this machine (GOFLAGS / GOPATH)
REM ============================================================

REM 1. Clean PATH (keep only system basics + Go bin)
set "PATH=C:\Windows\System32;C:\Windows;C:\Windows\System32\Wbem"

REM 2. Load MSVC
call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat"
if errorlevel 1 (
    echo [setenv] vcvars64 failed
    exit /b 1
)

REM 3. Project paths (this machine)
set "SW_ROOT=D:\code\shadow-worker"
set "PATH=D:\software\Go\bin;D:\Qt\6.11.1\msvc2022_64\bin;D:\Qt\Tools\CMake_64\bin;D:\Qt\Tools\Ninja;%PATH%"

REM 4. Fix polluted Go env (GOFLAGS has stray ";", GOPATH has wrong "\bin" suffix)
set "GOPATH=D:\software\Go"
set "GOBIN=D:\software\Go\bin"
set "GOFLAGS="

REM 5. Verify key tools
where cl >nul 2>&1 || (echo [setenv] cl.exe missing & exit /b 1)
where protoc >nul 2>&1 || (echo [setenv] protoc missing & exit /b 1)
where ninja >nul 2>&1 || (echo [setenv] ninja missing & exit /b 1)
where go >nul 2>&1 || (echo [setenv] go missing & exit /b 1)

echo [setenv] OK: cl / protoc / ninja / go ready
