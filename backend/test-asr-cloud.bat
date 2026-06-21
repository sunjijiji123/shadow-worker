@echo off
REM 运行 ASR 包的单元测试（需要 CGO 环境，因为 asr 包含 whisper 绑定）。
setlocal

set "W64=D:\Qt\w64devkit"
set "WSP=%~dp0third_party\whisper.cpp"
set "SCRIPT_DIR=%~dp0"

set "WHISPER_GO=%SCRIPT_DIR%vendor\github.com\ggerganov\whisper.cpp\bindings\go\whisper.go"
findstr /C:"-lgomp" "%WHISPER_GO%" >nul 2>&1 && goto :lgomp_present
powershell -NoProfile -ExecutionPolicy Bypass -Command "$f=$env:WHISPER_GO; $old='-lwhisper -lggml -lggml-base -lggml-cpu -lm -lstdc++'; $new='-lwhisper -lggml -lggml-base -lggml-cpu -lgomp -lm -lstdc++'; $c=Get-Content $f -Raw; $c=$c.Replace($old,$new); Set-Content -Path $f -Value $c -NoNewline"
echo [patch] added -lgomp
goto :lgomp_done
:lgomp_present
echo [patch] -lgomp already present
:lgomp_done

set "PATH=%W64%\bin;%PATH%"
set "CGO_ENABLED=1"
set "CC=%W64%\bin\gcc.exe"
set "CXX=%W64%\bin\g++.exe"
set "C_INCLUDE_PATH=%WSP%\include;%WSP%\ggml\include"
set "LIBRARY_PATH=%WSP%\build\src;%WSP%\build\ggml\src;%W64%\lib\gcc\x86_64-w64-mingw32\13.2.0"

pushd "%SCRIPT_DIR%"
go test -run "TestCloud|TestWrapWAV|TestNewCloud" -v ./internal/asr/ -timeout 60s
set "RC=%ERRORLEVEL%"
popd

endlocal & exit /b %RC%
