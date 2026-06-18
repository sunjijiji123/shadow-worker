@echo off
REM 临时调试脚本: 编译 + 跑客户端,直接看控制台输出
cd /d C:\Users\Administrator\code\1-ai\shadow-worker\client
set "PATH=C:\Qt\6.11.1\msvc2022_64\bin;C:\Windows\System32;C:\Windows;C:\Qt\Tools\CMake_64\bin;C:\Qt\Tools\Ninja"
call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat" >nul 2>&1

echo === 编译 ===
cmake --build build 2>&1
if errorlevel 1 (echo BUILD_FAILED & exit /b 1)

echo.
echo === 运行(8秒后强杀) ===
cd build
start /b shadow-worker-client.exe > run_stdout.log 2> run_stderr.log
ping -n 9 127.0.0.1 >nul
taskkill /im shadow-worker-client.exe /f >nul 2>&1

echo --- stdout ---
type run_stdout.log
echo.
echo --- stderr ---
type run_stderr.log
