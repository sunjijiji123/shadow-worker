@echo off
REM 生成 Go 的 gRPC 桩(Qt 端由 CMake qt_add_protobuf 自动生成)
REM 用法: scripts\gen_proto.bat

setlocal

set ROOT=%~dp0..
set PROTO_DIR=%ROOT%\proto
set PROTOC=%ROOT%\tools\protoc\bin\protoc.exe

REM Go 输出
set GO_OUT=%ROOT%\backend\internal\grpcapi

REM Qt 输出(C++)
set QT_OUT=%ROOT%\client\src\grpc

echo === 生成 Go gRPC 桩 ===
%PROTOC% -I %PROTO_DIR% ^
  --go_out=%GO_OUT% --go_opt=paths=source_relative ^
  --go-grpc_out=%GO_OUT% --go-grpc_opt=paths=source_relative ^
  %PROTO_DIR%\overview.proto

if errorlevel 1 (
  echo [FAIL] Go 桩生成失败
  exit /b 1
)

echo === 生成 Qt C++ 桩 ===
REM Qt 用 protoc 生成 .pb.cc/.pb.h,Qt6Grpc 的 QtProtobufCommon CMake 函数会接管
REM 这里先只生成 Go,Qt 端在 CMakeLists 里用 qt_add_protobuf 自动生成
echo [SKIP] Qt 端由 CMake qt_add_protobuf 自动生成,本脚本只生成 Go 桩

echo === 完成 ===
echo Go 桩: %GO_OUT%
echo Qt 桩: 由 client\CMakeLists.txt 自动生成

endlocal
