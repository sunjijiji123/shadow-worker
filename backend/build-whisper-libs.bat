@echo off
REM Build whisper.cpp static libraries with MinGW (w64devkit gcc 13.2.0) + Ninja.
REM Produces:
REM   third_party\whisper.cpp\build\src\libwhisper.a
REM   third_party\whisper.cpp\build\ggml\src\libggml.a, libggml-base.a, libggml-cpu.a
REM
REM Prerequisites:
REM   - D:\Qt\w64devkit       (gcc 13.2.0; gcc 16 produces invalid COFF for Go cgo)
REM   - D:\Qt\Tools\Ninja     (CMake generator)
REM   - CMake on PATH
REM   - third_party\whisper.cpp checked out (see notes in WHISPER_BUILD.md)
REM
REM Notes:
REM   - GGML_OPENMP=ON is required for multi-threaded inference; the Go cgo
REM     link step needs -lgomp (handled by build-whisper-cgo.bat).
REM   - A small source patch works around a missing THREAD_POWER_THROTTLING_*
REM     API in the w64devkit MinGW headers. It is applied idempotently below.

setlocal

set "W64=D:\Qt\w64devkit"
set "NINJA=D:\Qt\Tools\Ninja"
set "SCRIPT_DIR=%~dp0"
set "WSP=%SCRIPT_DIR%third_party\whisper.cpp"

if not exist "%WSP%\CMakeLists.txt" (
    echo [FAIL] whisper.cpp source not found at %WSP%
    echo        Clone it: git clone https://github.com/ggerganov/whisper.cpp "%WSP%"
    exit /b 1
)

set "PATH=%W64%\bin;%NINJA%;%PATH%"

REM --- Apply MinGW throttling-API source patch (idempotent) -----------------
set "CPU_C=%WSP%ggml\src\ggml-cpu\ggml-cpu.c"
findstr /C:"THREAD_POWER_THROTTLING_CURRENT_VERSION" "%CPU_C%" >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    echo [patch] throttling fallback already present
) else (
    echo [FAIL] throttling fallback block not found in ggml-cpu.c
    echo        This patch is normally applied manually; see WHISPER_BUILD.md.
    exit /b 1
)

REM --- Configure + build ---------------------------------------------------
if exist "%WSP%\build" rmdir /s /q "%WSP%\build"

echo [cmake] configuring...
cmake -S "%WSP%" -B "%WSP%\build" -G Ninja ^
    -DCMAKE_C_COMPILER=%W64%/bin/gcc.exe ^
    -DCMAKE_CXX_COMPILER=%W64%/bin/g++.exe ^
    -DCMAKE_MAKE_PROGRAM=%NINJA%/ninja.exe ^
    -DCMAKE_BUILD_TYPE=Release ^
    -DWHISPER_BUILD_TESTS=OFF ^
    -DWHISPER_BUILD_EXAMPLES=OFF ^
    -DGGML_OPENMP=ON ^
    -DBUILD_SHARED_LIBS=OFF
if %ERRORLEVEL% NEQ 0 (
    echo [FAIL] cmake configure failed
    exit /b 1
)

echo [cmake] building whisper target...
cmake --build "%WSP%\build" --target whisper -j
if %ERRORLEVEL% NEQ 0 (
    echo [FAIL] cmake build failed
    exit /b 1
)

REM --- Rename ggml libs to lib*.a (cgo -lggml expects lib prefix) -----------
pushd "%WSP%\build\ggml\src"
if exist ggml.a       ren ggml.a       libggml.a
if exist ggml-base.a  ren ggml-base.a  libggml-base.a
if exist ggml-cpu.a   ren ggml-cpu.a   libggml-cpu.a
popd

echo [OK] whisper static libs built in %WSP%\build
endlocal & exit /b 0
