@echo off
REM Shadow Worker update service build script
REM Produces: dist/update-service.exe

set "ROOT=%~dp0.."
if not exist "%ROOT%\dist" mkdir "%ROOT%\dist"

cd /d "%ROOT%"
go build -ldflags="-s -w" -o dist/update-service.exe ./cmd/server
if errorlevel 1 (
    echo [FAIL] update service build failed
    exit /b 1
)

echo [OK] update-service: %ROOT%\dist\update-service.exe
