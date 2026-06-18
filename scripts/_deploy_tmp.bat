@echo off
cd /d C:\Users\Administrator\code\1-ai\shadow-worker\client\build
set PATH=C:\Qt\6.11.1\msvc2022_64\bin;C:\Windows\System32;C:\Windows
call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat" >nul 2>&1
echo --- windeployqt ---
windeployqt --qmldir ..\qml shadow-worker-client.exe
echo --- deploy done ---
dir Qt6Core.dll
