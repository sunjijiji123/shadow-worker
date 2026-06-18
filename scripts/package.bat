@echo off
chcp 65001 >nul
setlocal EnableDelayedExpansion

call "%~dp0..\scripts\setenv.bat"

set "ROOT=%~dp0.."
set "DIST=%ROOT%\dist"
set "VERSION=0.1.0"

echo [package] clean old dist...
if exist "%DIST%" rmdir /S /Q "%DIST%"
mkdir "%DIST%\bin"
mkdir "%DIST%\config"

echo [package] build backend...
cd /d "%ROOT%\backend"
go build -ldflags="-s -w" -o "%DIST%\bin\shadow-worker.exe" ./cmd/shadow-worker
if errorlevel 1 (
    echo [package] backend build failed
    exit /b 1
)

echo [package] build Qt client Release...
cd /d "%ROOT%\client"
cmake -B build -S . -G Ninja -DCMAKE_BUILD_TYPE=Release >nul 2>&1
cmake --build build
if errorlevel 1 (
    echo [package] client build failed
    exit /b 1
)

copy /Y "%ROOT%\client\build\shadow-worker-client.exe" "%DIST%\bin\" >nul

echo [package] deploy Qt deps...
windeployqt "%DIST%\bin\shadow-worker-client.exe" --release --no-translations --no-compiler-runtime

echo [package] copy VC runtime...
if defined VCToolsRedistDir (
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\msvcp140.dll" "%DIST%\bin\" >nul
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\vcruntime140.dll" "%DIST%\bin\" >nul
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\vcruntime140_1.dll" "%DIST%\bin\" >nul
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\concrt140.dll" "%DIST%\bin\" >nul
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\msvcp140_1.dll" "%DIST%\bin\" >nul
    copy /Y "%VCToolsRedistDir%\x64\Microsoft.VC143.CRT\msvcp140_2.dll" "%DIST%\bin\" >nul
) else (
    echo [package] warning: VCToolsRedistDir not set, VC runtime may be missing
)

echo [package] copy default config...
if exist "%ROOT%\backend\config.yaml" (
    copy /Y "%ROOT%\backend\config.yaml" "%DIST%\config\" >nul
)

echo [package] build installer...
where iscc >nul 2>&1
if errorlevel 1 (
    echo [package] Inno Setup compiler ^(iscc.exe^) not found, skip installer
    echo [package] portable files ready: %DIST%
    exit /b 0
)
:compile
set "ISCC=C:\Program Files (x86)\Inno Setup 6\ISCC.exe"
if not exist "%ISCC%" set "ISCC=iscc.exe"
"%ISCC%" "%ROOT%\package\ShadowWorker.iss"
if errorlevel 1 (
    echo [package] Inno Setup compile failed
    exit /b 1
)

echo [package] done: %DIST%
