@echo off
REM 运行 ASR 本地引擎端到端测试（whisper.cpp CGO + jfk.wav + ggml-small.bin）。
REM 用法: test-asr-e2e.bat
REM 前置: 同 build-whisper-cgo.bat 的 CGO 工具链要求。

setlocal

set "W64=D:\Qt\w64devkit"
set "WSP=%~dp0third_party\whisper.cpp"
set "SCRIPT_DIR=%~dp0"

REM vendor patch（同 build-whisper-cgo.bat）
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
go test -tags asr_e2e -run TestLocalEngineE2E -v ./internal/asr/ -timeout 180s
set "RC=%ERRORLEVEL%"
popd

endlocal & exit /b %RC%
