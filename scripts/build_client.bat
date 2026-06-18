@echo off
REM ============================================================
REM Shadow Worker Qt 客户端构建脚本
REM ============================================================
REM 干净的 MSVC 构建环境(排除 MSYS2 干扰)。
REM
REM 用法:
REM   scripts\build_client.bat            配置 + 编译
REM   scripts\build_client.bat clean      清理后重新配置 + 编译
REM   scripts\build_client.bat run        编译后运行客户端
REM ============================================================

setlocal enabledelayedexpansion

REM --- 路径常量(按需改) ---
set "QT_PREFIX=C:\Qt\6.11.1\msvc2022_64"
set "QT_TOOLS=C:\Qt\Tools"
set "VS_VCVARS=C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat"
set "PROTOC_DIR=%~dp0..\tools\protoc\bin"
set "CLIENT_DIR=%~dp0..\client"

REM --- 1. 清理 PATH(移除 MSYS2/MinGW 干扰) ---
REM 把含 msys64 或 mingw 的路径项全部剔除,避免用错编译器
set "CLEAN_PATH="
for %%P in ("%PATH:;=";"%") do (
    set "ITEM=%%~P"
    echo !ITEM! | findstr /i "msys64 mingw" >nul
    if errorlevel 1 (
        if defined CLEAN_PATH (
            set "CLEAN_PATH=!CLEAN_PATH!;%%~P"
        ) else (
            set "CLEAN_PATH=%%~P"
        )
    )
)
set "PATH=%CLEAN_PATH%"
echo [1/4] PATH 已清理(移除 MSYS2/MinGW)

REM --- 2. 加入 Qt 工具 + 项目 protoc ---
set "PATH=%QT_TOOLS%\CMake_64\bin;%QT_TOOLS%\Ninja;%PROTOC_DIR%;%PATH%"
echo [2/4] 已加入 Qt CMake/Ninja + 项目 protoc

REM --- 3. 调用 MSVC 环境 ---
if not exist "%VS_VCVARS%" (
    echo [错误] 找不到 vcvars64: %VS_VCVARS%
    exit /b 1
)
call "%VS_VCVARS%" >nul 2>&1
echo [3/4] MSVC 环境已加载

REM --- 4. 验证关键工具 ---
where cl >nul 2>&1 || (echo [错误] cl.exe 不在 PATH,vcvars64 可能失败 & exit /b 1)
where protoc >nul 2>&1 || (echo [错误] protoc 不在 PATH & exit /b 1)
where ninja >nul 2>&1 || (echo [错误] ninja 不在 PATH & exit /b 1)
echo [4/4] 工具验证通过: cl / protoc / ninja 就绪

REM --- 配置(可选 clean) ---
cd /d "%CLIENT_DIR%"
if /i "%1"=="clean" (
    echo --- 清理 build 目录 ---
    rmdir /s /q build 2>nul
)

if not exist build\CMakeCache.txt (
    echo --- CMake 配置 ---
    cmake -B build -S . -G Ninja -DCMAKE_PREFIX_PATH="%QT_PREFIX%" -DCMAKE_BUILD_TYPE=Debug
    if errorlevel 1 (echo [错误] CMake 配置失败 & exit /b 1)
)

REM --- 编译 ---
echo --- 编译 ---
cmake --build build
if errorlevel 1 (echo [错误] 编译失败 & exit /b 1)

echo.
echo === 构建成功 ===
echo 输出: %CLIENT_DIR%\build\shadow-worker-client.exe

REM --- 可选运行 ---
if /i "%1"=="run" (
    echo --- 运行客户端 ---
    "%CLIENT_DIR%\build\shadow-worker-client.exe"
)

endlocal
