@echo off
REM Build the shadow-worker backend with whisper.cpp CGO linking.
REM Usage: build-whisper-cgo.bat [output-path]
REM
REM Prerequisites:
REM   - D:\Qt\w64devkit          (gcc 13.2.0; gcc 16 produces invalid COFF for Go cgo)
REM   - D:\Qt\Tools\Ninja        (whisper.cpp CMake build generator)
REM   - whisper.cpp static libs  (see build-whisper-libs.bat if rebuild needed)
REM
REM This script:
REM   1. (Re)applies the vendor patch that adds -lgomp to the whisper cgo LDFLAGS.
REM      Required because `go mod vendor` wipes the edit; without it the link
REM      step fails with undefined references to GOMP_*/omp_* symbols.
REM      Idempotent: a no-op when the line is already present.
REM   2. Sets up the CGO toolchain env (CC/CXX, include/lib search paths).
REM   3. Runs `go build`.

setlocal

set "W64=D:\Qt\w64devkit"
set "WSP=%~dp0third_party\whisper.cpp"
set "SCRIPT_DIR=%~dp0"

REM --- 1. Ensure vendor whisper.go has -lgomp -------------------------------
REM The upstream binding only adds -fopenmp on linux; on windows/mingw the
REM ggml static libs reference libgomp symbols that need explicit linking.
set "WHISPER_GO=%SCRIPT_DIR%vendor\github.com\ggerganov\whisper.cpp\bindings\go\whisper.go"

if not exist "%WHISPER_GO%" (
    echo [FAIL] vendor whisper.go not found at %WHISPER_GO%
    echo        Run "go mod vendor" in backend\ first.
    exit /b 1
)

findstr /C:"-lgomp" "%WHISPER_GO%" >nul 2>&1 && goto :lgomp_present

REM Not present: patch via PowerShell (idempotent replace).
powershell -NoProfile -ExecutionPolicy Bypass -Command "$f=$env:WHISPER_GO; $old='-lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++'; $new='-lwhisper -lggml -lggml-base -lggml-cpu -lgomp -lm -lstdc++'; $c=Get-Content $f -Raw; $c=$c.Replace($old,$new); Set-Content -Path $f -Value $c -NoNewline"
if %ERRORLEVEL% NEQ 0 (
    echo [FAIL] could not patch vendor whisper.go
    exit /b 1
)
echo [patch] added -lgomp to whisper cgo LDFLAGS
goto :lgomp_done

:lgomp_present
echo [patch] -lgomp already present in whisper.go

:lgomp_done

REM --- 2. CGO toolchain env -------------------------------------------------
set "PATH=%W64%\bin;%PATH%"
set "CGO_ENABLED=1"
set "CC=%W64%\bin\gcc.exe"
set "CXX=%W64%\bin\g++.exe"
set "C_INCLUDE_PATH=%WSP%\include;%WSP%\ggml\include"
set "LIBRARY_PATH=%WSP%\build\src;%WSP%\build\ggml\src;%W64%\lib\gcc\x86_64-w64-mingw32\13.2.0"

REM --- 3. go build ----------------------------------------------------------
set "OUT=%~1"
if "%OUT%"=="" set "OUT=%SCRIPT_DIR%..\build\shadow-worker.exe"

pushd "%SCRIPT_DIR%"
go build -o "%OUT%" ./cmd/shadow-worker/
set "RC=%ERRORLEVEL%"
popd

if "%RC%"=="0" (
    echo [OK] build succeeded: %OUT%
) else (
    echo [FAIL] go build returned %RC%
)
endlocal & exit /b %RC%
