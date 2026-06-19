cmake_minimum_required(VERSION 3.21)

# ============================================================
# Shadow Worker - Qt 客户端
# ============================================================
# 构建前请设置环境(在开发 shell 里):
#   set CMAKE_PREFIX_PATH=C:\Qt\6.11.1\msvc2022_64
#   "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat"
#
# 配置:
#   cmake -B build -S . -G Ninja
# 编译:
#   cmake --build build
# 运行:
#   build\shadow-worker-client.exe
# ============================================================

# Locate protoc for WrapProtoc.
# Priority: project-bundled tools/protoc > system PATH (this machine: D:\software\Go\bin\protoc.exe).
set(PROTOC_PATH "${CMAKE_SOURCE_DIR}/../tools/protoc/bin/protoc.exe")
if(EXISTS "${PROTOC_PATH}")
    set(Protobuf_PROTOC_EXECUTABLE "${PROTOC_PATH}" CACHE FILEPATH "protoc location")
    message(STATUS "Using bundled protoc: ${PROTOC_PATH}")
else()
    # fall back to protoc on PATH (this machine's Go bin dir is on PATH)
    find_program(SYSTEM_PROTOC protoc)
    if(SYSTEM_PROTOC)
        set(Protobuf_PROTOC_EXECUTABLE "${SYSTEM_PROTOC}" CACHE FILEPATH "protoc location")
        message(STATUS "Using system protoc: ${SYSTEM_PROTOC}")
    else()
        message(WARNING "protoc not found; ensure it is on PATH (this machine: D:\\software\\Go\\bin)")
    endif()
endif()

project(shadow-worker-client LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_AUTOMOC ON)
set(CMAKE_AUTORCC ON)

# Qt6 模块
find_package(Qt6 REQUIRED COMPONENTS
    Core
    Gui
    Quick            # QML 运行时
    QuickControls2   # QML 控件
    QuickLayouts     # QML 布局
    Widgets          # 托盘/QSystemTrayIcon
    Multimedia       # 麦克风
    Grpc             # gRPC 客户端
    Protobuf         # protobuf 消息
    ProtobufTools    # qt_add_protobuf 函数
)

qt_standard_project_setup(REQUIRES 6.5)

# ============================================================
# 从 proto 生成 C++ 桩(Qt6Grpc 原生支持)
# ============================================================
set(PROTO_DIR "${CMAKE_SOURCE_DIR}/../proto")
set(GEN_DIR "${CMAKE_BINARY_DIR}/gen_proto")

# overview
qt_add_protobuf(overview_proto
    PROTO_FILES ${PROTO_DIR}/overview.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
qt_add_grpc(overview_grpc CLIENT
    PROTO_FILES ${PROTO_DIR}/overview.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
target_link_libraries(overview_grpc PRIVATE overview_proto)

# whitelist
qt_add_protobuf(whitelist_proto
    PROTO_FILES ${PROTO_DIR}/whitelist.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
qt_add_grpc(whitelist_grpc CLIENT
    PROTO_FILES ${PROTO_DIR}/whitelist.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
target_link_libraries(whitelist_grpc PRIVATE whitelist_proto)

# asr
qt_add_protobuf(asr_proto
    PROTO_FILES ${PROTO_DIR}/asr.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
qt_add_grpc(asr_grpc CLIENT
    PROTO_FILES ${PROTO_DIR}/asr.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
target_link_libraries(asr_grpc PRIVATE asr_proto)

# collection (timeline / control)
qt_add_protobuf(collection_proto
    PROTO_FILES ${PROTO_DIR}/collection.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
qt_add_grpc(collection_grpc CLIENT
    PROTO_FILES ${PROTO_DIR}/collection.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
target_link_libraries(collection_grpc PRIVATE collection_proto whitelist_proto)

# config
qt_add_protobuf(config_proto
    PROTO_FILES ${PROTO_DIR}/config.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
qt_add_grpc(config_grpc CLIENT
    PROTO_FILES ${PROTO_DIR}/config.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
target_link_libraries(config_grpc PRIVATE config_proto whitelist_proto)

# ============================================================
# 源文件
# ============================================================
qt_add_executable(shadow-worker-client
    src/main.cpp
    src/viewmodels/overview_vm.h
    src/viewmodels/overview_vm.cpp
    src/viewmodels/whitelist_vm.h
    src/viewmodels/whitelist_vm.cpp
    src/viewmodels/timeline_vm.h
    src/viewmodels/timeline_vm.cpp
    src/viewmodels/settings_vm.h
    src/viewmodels/settings_vm.cpp
    src/window/windowpicker.h
    src/window/windowpicker.cpp
    src/audio/audiorecorder.h
    src/audio/audiorecorder.cpp
    src/hotkey/globalhotkey.h
    src/hotkey/globalhotkey.cpp
    src/asr/asrclient.h
    src/asr/asrclient.cpp
    src/utils/autostart.h
    src/utils/autostart.cpp
)

# 链接 Qt 模块 + 生成的 protobuf/grpc 桩
target_link_libraries(shadow-worker-client PRIVATE
    Qt6::Core
    Qt6::Gui
    Qt6::Quick
    Qt6::QuickControls2
    Qt6::QuickLayouts
    Qt6::Widgets
    Qt6::Multimedia
    Qt6::Grpc
    overview_proto
    overview_grpc
    whitelist_proto
    whitelist_grpc
    asr_proto
    asr_grpc
    collection_proto
    collection_grpc
    config_proto
    config_grpc
)

# QML 文件作为资源
target_include_directories(shadow-worker-client PRIVATE
    ${GEN_DIR}
    ${CMAKE_SOURCE_DIR}/src
    ${CMAKE_SOURCE_DIR}/src/viewmodels
    ${CMAKE_SOURCE_DIR}/src/window
    ${CMAKE_SOURCE_DIR}/src/audio
    ${CMAKE_SOURCE_DIR}/src/hotkey
    ${CMAKE_SOURCE_DIR}/src/asr
    ${CMAKE_SOURCE_DIR}/src/utils
)

qt_add_qml_module(shadow-worker-client
    URI ShadowWorker
    VERSION 1.0
    QML_FILES
        # theme (pragma singleton -> auto-registered as singleton in main module)
        qml/theme/Theme.qml
        # components
        qml/components/Card.qml
        qml/components/Toggle.qml
        qml/components/Radio.qml
        qml/components/Chip.qml
        qml/components/Button.qml
        qml/components/StatCard.qml
        qml/components/Toast.qml
        qml/components/SaveBar.qml
        qml/components/PlaceholderLabel.qml
        # pages
        qml/main.qml
        qml/pages/OverviewPage.qml
        qml/pages/TimelinePage.qml
        qml/pages/SettingsPage.qml
        qml/pages/SystemPage.qml
)

# Windows: keep console in DEBUG builds (to see errors); RELEASE would use WIN32 to hide it.
set_target_properties(shadow-worker-client PROPERTIES
    WIN32_EXECUTABLE OFF
)

# Install rules (for packaging)
install(TARGETS shadow-worker-client
    BUNDLE DESTINATION .
    RUNTIME DESTINATION bin
)
