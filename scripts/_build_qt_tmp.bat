@echo off
call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat" >nul 2>&1
cd /d D:\code\shadow-worker\client
D:\Qt\Tools\CMake_64\bin\cmake.exe -B build
if errorlevel 1 exit /b 1
D:\Qt\Tools\CMake_64\bin\cmake.exe --build build --config Debug
exit /b %ERRORLEVEL%
