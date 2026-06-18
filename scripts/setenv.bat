@echo off
REM ============================================================
REM Shadow Worker 开发环境加载
REM ============================================================
REM 用法: 开发前先跑这个脚本
REM   C:\Users\Administrator\code\1-ai\shadow-worker\scripts\setenv.bat
REM 之后 go build / cmake --build 都能直接用
REM
REM 作用:
REM   1. 清理 PATH 中的 MSYS2/MinGW(避免污染 MSVC 链接)
REM   2. 加载 MSVC 环境(vcvars64,设 LIB/INCLUDE)
REM   3. 把 Qt6 MSVC bin / CMake / Ninja / 项目 protoc 放 PATH 最前
REM ============================================================

REM 1. 清 PATH(只留系统基础)
set "PATH=C:\Windows\System32;C:\Windows;C:\Windows\System32\Wbem"

REM 2. 加载 MSVC
call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat"
if errorlevel 1 (
    echo [setenv] vcvars64 失败
    exit /b 1
)

REM 3. 项目路径常量
set "SW_ROOT=%~dp0.."
set "PATH=%SW_ROOT%\tools\protoc\bin;C:\Qt\6.11.1\msvc2022_64\bin;C:\Qt\Tools\CMake_64\bin;C:\Qt\Tools\Ninja;C:\Program Files\Go1.26.1\bin;%PATH%"

REM 4. 验证关键工具
where cl >nul 2>&1 || (echo [setenv] cl.exe 缺失 & exit /b 1)
where protoc >nul 2>&1 || (echo [setenv] protoc 缺失 & exit /b 1)
where ninja >nul 2>&1 || (echo [setenv] ninja 缺失 & exit /b 1)
where go >nul 2>&1 || (echo [setenv] go 缺失 & exit /b 1)

echo [setenv] OK: cl / protoc / ninja / go 就绪
