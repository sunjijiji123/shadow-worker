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
    Network          # QNetworkAccessManager（更新包下载）
    Grpc             # gRPC 客户端
    Protobuf         # protobuf 消息
    ProtobufTools    # qt_add_protobuf 函数
    LinguistTools    # qt_add_translations / lupdate / lrelease
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

# update
qt_add_protobuf(update_proto
    PROTO_FILES ${PROTO_DIR}/update.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
qt_add_grpc(update_grpc CLIENT
    PROTO_FILES ${PROTO_DIR}/update.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
target_link_libraries(update_grpc PRIVATE update_proto)

# voice (recording control + live spectrum stream)
qt_add_protobuf(voice_proto
    PROTO_FILES ${PROTO_DIR}/voice.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
qt_add_grpc(voice_grpc CLIENT
    PROTO_FILES ${PROTO_DIR}/voice.proto
    OUTPUT_DIRECTORY ${GEN_DIR}
)
target_link_libraries(voice_grpc PRIVATE voice_proto)
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
    src/viewmodels/timelinemodels.h
    src/viewmodels/timelinemodels.cpp
    src/viewmodels/categoryproxy.h
    src/viewmodels/categoryproxy.cpp
    src/viewmodels/settings_vm.h
    src/viewmodels/settings_vm.cpp
    src/viewmodels/update_vm.h
    src/viewmodels/update_vm.cpp
    src/window/windowpicker.h
    src/window/windowpicker.cpp
    src/window/windowhelper.h
    src/window/windowhelper.cpp
    src/window/screenshotwindow.h
    src/window/screenshotwindow.cpp
    src/window/screenshotcontroller.h
    src/window/screenshotcontroller.cpp
    src/window/textinjector.h
    src/window/textinjector.cpp
    src/audio/audiorecorder.h
    src/audio/audiorecorder.cpp
    src/audio/audiodevicemanager.h
    src/audio/audiodevicemanager.cpp
    src/hotkey/globalhotkey.h
    src/hotkey/globalhotkey.cpp
    src/ui/traycontroller.h
    src/ui/traycontroller.cpp
    src/asr/asrclient.h
    src/asr/asrclient.cpp
    src/asr/voiceclient.h
    src/asr/voiceclient.cpp
    src/asr/collectionclient.h
    src/asr/collectionclient.cpp
    src/utils/autostart.h
    src/utils/autostart.cpp
    src/utils/singleinstance.h
    src/utils/singleinstance.cpp
    src/utils/backendlauncher.h
    src/utils/backendlauncher.cpp
    src/i18n/translator.h
    src/i18n/translator.cpp
    src/app.rc
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
    Qt6::Network
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
    update_proto
    update_grpc
    voice_proto
    voice_grpc
)

# QML 文件作为资源
target_include_directories(shadow-worker-client PRIVATE
    ${GEN_DIR}
    ${CMAKE_SOURCE_DIR}/src
    ${CMAKE_SOURCE_DIR}/src/viewmodels
    ${CMAKE_SOURCE_DIR}/src/window
    ${CMAKE_SOURCE_DIR}/src/audio
    ${CMAKE_SOURCE_DIR}/src/hotkey
    ${CMAKE_SOURCE_DIR}/src/ui
    ${CMAKE_SOURCE_DIR}/src/asr
    ${CMAKE_SOURCE_DIR}/src/utils
    ${CMAKE_SOURCE_DIR}/src/i18n
)

# Declare Theme.qml as a singleton type (Qt6 way; no `pragma singleton` in the file).
set_source_files_properties(qml/theme/Theme.qml
    PROPERTIES QT_QML_SINGLETON_TYPE TRUE)

qt_add_qml_module(shadow-worker-client
    URI ShadowWorker
    VERSION 1.0
    QML_FILES
        # theme (singleton, declared above via QT_QML_SINGLETON_TYPE)
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
        qml/components/HeatmapGrid.qml
        qml/components/RankBar.qml
        qml/components/QuickSwitchRow.qml
        qml/components/DatePicker.qml
        qml/components/TimelineTrack.qml
        qml/components/TextField.qml
        qml/components/SelectBox.qml
        qml/components/ModelChipGroup.qml
        qml/components/VolumeBar.qml
        qml/components/MicTestButton.qml
        qml/components/AddModelDialog.qml
        qml/components/AddAppDialog.qml
        qml/components/ConfirmDialog.qml
        qml/components/TextArea.qml
        qml/components/PromptItem.qml
        qml/components/KeyCap.qml
        qml/components/Badge.qml
        qml/components/TitleBar.qml
        qml/components/RecordingBubble.qml
        qml/components/ResultBubble.qml
        qml/components/UpdateBadge.qml
        qml/components/UpdateDialog.qml
        qml/RecordingWindow.qml
        qml/ScreenshotResultWindow.qml
        # pages
        qml/main.qml
        qml/pages/OverviewPage.qml
        qml/pages/TimelinePage.qml
        qml/pages/SettingsPage.qml
        qml/pages/SystemPage.qml
    RESOURCES
        qml/icons/overview.svg
        qml/icons/overview_active.svg
        qml/icons/timeline.svg
        qml/icons/timeline_active.svg
        qml/icons/settings.svg
        qml/icons/settings_active.svg
        qml/icons/system.svg
        qml/icons/system_active.svg
        qml/icons/toast_success.svg
        qml/icons/toast_error.svg
        qml/icons/toast_warning.svg
        qml/icons/polish.svg
        qml/icons/polish_active.svg
    RESOURCES
        assets/app.ico
)

# ============================================================
# i18n: translations (.ts source -> .qm compiled, embedded in qrc)
# ============================================================
# Step 1: qt_add_translations runs lupdate (scan qsTr) + lrelease (.ts->.qm).
#         QM_FILES_OUTPUT_VARIABLE gives us the list of generated .qm paths.
# Step 2: qt_add_resources embeds the .qm into the binary's qrc.
#         BASE is set to the binary dir so absolute .qm paths are resolved
#         to relative paths (qt_add_resources requires relative paths).
# Source strings are English; "en" needs no .qm (source shows through).
# Translator.cpp loads :/i18n/shadowworker_zh_CN.qm at runtime.
qt_add_translations(shadow-worker-client
    TS_FILES
        ${CMAKE_SOURCE_DIR}/i18n/shadowworker_zh_CN.ts
    SOURCES
        # C++ 源（tr() 翻译来源，如托盘菜单项）
        ${CMAKE_SOURCE_DIR}/src/ui/traycontroller.cpp
        ${CMAKE_SOURCE_DIR}/qml/main.qml
        ${CMAKE_SOURCE_DIR}/qml/RecordingWindow.qml
        ${CMAKE_SOURCE_DIR}/qml/ScreenshotResultWindow.qml
        ${CMAKE_SOURCE_DIR}/qml/theme/Theme.qml
        ${CMAKE_SOURCE_DIR}/qml/components/Card.qml
        ${CMAKE_SOURCE_DIR}/qml/components/Toggle.qml
        ${CMAKE_SOURCE_DIR}/qml/components/Radio.qml
        ${CMAKE_SOURCE_DIR}/qml/components/Chip.qml
        ${CMAKE_SOURCE_DIR}/qml/components/Button.qml
        ${CMAKE_SOURCE_DIR}/qml/components/StatCard.qml
        ${CMAKE_SOURCE_DIR}/qml/components/Toast.qml
        ${CMAKE_SOURCE_DIR}/qml/components/SaveBar.qml
        ${CMAKE_SOURCE_DIR}/qml/components/PlaceholderLabel.qml
        ${CMAKE_SOURCE_DIR}/qml/components/HeatmapGrid.qml
        ${CMAKE_SOURCE_DIR}/qml/components/RankBar.qml
        ${CMAKE_SOURCE_DIR}/qml/components/QuickSwitchRow.qml
        ${CMAKE_SOURCE_DIR}/qml/components/DatePicker.qml
        ${CMAKE_SOURCE_DIR}/qml/components/TimelineTrack.qml
        ${CMAKE_SOURCE_DIR}/qml/components/TextField.qml
        ${CMAKE_SOURCE_DIR}/qml/components/SelectBox.qml
        ${CMAKE_SOURCE_DIR}/qml/components/ModelChipGroup.qml
        ${CMAKE_SOURCE_DIR}/qml/components/VolumeBar.qml
        ${CMAKE_SOURCE_DIR}/qml/components/MicTestButton.qml
        ${CMAKE_SOURCE_DIR}/qml/components/AddModelDialog.qml
        ${CMAKE_SOURCE_DIR}/qml/components/AddAppDialog.qml
        ${CMAKE_SOURCE_DIR}/qml/components/ConfirmDialog.qml
        ${CMAKE_SOURCE_DIR}/qml/components/TextArea.qml
        ${CMAKE_SOURCE_DIR}/qml/components/PromptItem.qml
        ${CMAKE_SOURCE_DIR}/qml/components/KeyCap.qml
        ${CMAKE_SOURCE_DIR}/qml/components/Badge.qml
        ${CMAKE_SOURCE_DIR}/qml/components/TitleBar.qml
        ${CMAKE_SOURCE_DIR}/qml/components/RecordingBubble.qml
        ${CMAKE_SOURCE_DIR}/qml/components/ResultBubble.qml
        ${CMAKE_SOURCE_DIR}/qml/components/UpdateBadge.qml
        ${CMAKE_SOURCE_DIR}/qml/components/UpdateDialog.qml
        ${CMAKE_SOURCE_DIR}/qml/pages/OverviewPage.qml
        ${CMAKE_SOURCE_DIR}/qml/pages/TimelinePage.qml
        ${CMAKE_SOURCE_DIR}/qml/pages/SettingsPage.qml
        ${CMAKE_SOURCE_DIR}/qml/pages/SystemPage.qml
    QM_FILES_OUTPUT_VARIABLE QM_FILES
)
qt_add_resources(shadow-worker-client "i18n"
    PREFIX "/i18n"
    BASE "${CMAKE_CURRENT_BINARY_DIR}"
    FILES ${QM_FILES}
)

# Windows GUI subsystem: hide console in Release, keep it in Debug.
# Release: WIN32_EXECUTABLE ON -> /SUBSYSTEM:WINDOWS (no console)
# Debug:   WIN32_EXECUTABLE OFF -> /SUBSYSTEM:CONSOLE (see stderr)
if(CMAKE_BUILD_TYPE STREQUAL "Release")
    set_target_properties(shadow-worker-client PROPERTIES WIN32_EXECUTABLE ON)
else()
    set_target_properties(shadow-worker-client PROPERTIES WIN32_EXECUTABLE OFF)
endif()

# Install rules (for packaging)
install(TARGETS shadow-worker-client
    BUNDLE DESTINATION .
    RUNTIME DESTINATION bin
)
