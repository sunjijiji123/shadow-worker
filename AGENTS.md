# AGENTS.md — AI Agent 构建指南

Shadow Worker 是一个本地行为跟踪桌面应用：Go 后端（gRPC + SQLite + whisper.cpp CGO）+ Qt/QML 前端。

## 构建命令

### Go 后端（含 whisper.cpp CGO）

```cmd
backend\build-whisper-cgo.bat
```

**必须用 w64devkit gcc 13.2.0**（`D:\Qt\w64devkit`），gcc 16 会产生 Go cgo 无法解析的 COFF。详见 `backend/WHISPER_BUILD.md`。

### Qt 客户端

```cmd
:: 方式 1：vcvars64 + cmake（推荐，可靠）
call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat"
cd client
D:\Qt\Tools\CMake_64\bin\cmake.exe --build build --config Debug

:: 方式 2：临时脚本
scripts\_build_qt_tmp.bat
```

**不能直接 `cmake --build`**——MSVC 的 `type_traits` 等头文件路径需要 vcvars64 初始化。

### 重新生成 proto

```cmd
:: Go stubs
set PATH=D:\software\Go\bin;%PATH%
protoc -I proto --go_out=backend/internal/grpcapi --go_opt=paths=source_relative --go-grpc_out=backend/internal/grpcapi --go-grpc_opt=paths=source_relative proto/voice.proto

:: Qt stubs 由 CMake qt_add_protobuf / qt_add_grpc 自动生成（cmake --build 时触发）
```

### 启动

```cmd
:: 后端
build\shadow-worker.exe

:: 客户端（从 build 目录启动，DLL 在那里）
cd client\build
shadow-worker-client.exe
```

配置文件：`%APPDATA%\shadow-worker\config.yaml`（`C:\Users\sun\AppData\Roaming\shadow-worker\config.yaml`）

### 测试

```cmd
:: ASR 单元测试（cloud httptest，不需要模型）
backend\test-asr-cloud.bat

:: ASR 端到端测试（需要 modules/*.bin 模型 + CGO 环境）
backend\test-asr-e2e.bat
```

## 踩过的坑（重要！）

### QML / Qt Quick

1. **自定义 TextField 的 `onTextEdited` 带参数 `newText`**：项目里的 `TextField` 是自定义组件（`components/TextField.qml`），signal 声明为 `signal textEdited(string newText)`。必须用 `onTextEdited: function(newText) { ... }` 接收。直接读 `text` 属性在编辑过程中是 **undefined/旧值**——不要依赖它。

2. **`text: property` 绑定 + `onTextEdited` 冲突**：如果 TextField 有 `text: someProperty` 绑定，用户打字时绑定会在 `onTextEdited` **之前**重新求值，用旧值覆盖用户输入。需要用户编辑的 TextField **不要加 `text:` 绑定**，改用 `updateFields()` 命令式赋值。

3. **`visible: false` 的组件不接收事件**：在 `visible` 切换的 GridLayout 里的 TextField，其 `onTextEdited` 可能在 visible=false 时不触发。

4. **`Component.onCompleted` 里同步修改子组件触发 `m_componentComplete` ASSERT**：用 `Qt.callLater(syncFunction)` 延迟到下一事件循环。

5. **SelectBox `currentIndex: 0` 是死绑定**：会导致下拉框永远选中第一项。去掉绑定，用 `onOpened`/`updateFields()` 命令式赋值。

6. **信号参数声明**：Qt6 要求 `onSelected: function(index, value) { ... }`，旧的 `onSelected: root.x = index` 参数注入已废弃。

7. **`.bat` 文件需要 CRLF 行尾**：Write 工具默认输出 LF，cmd.exe 解析 LF 的 `.bat` 会报"系统找不到指定的路径"。用 `sed -i 's/$/\r/'` 转换。

8. **windeployqt 部署**：新增 QML import（如 `QtQuick.Dialogs`）后，build 目录可能缺 DLL。跑一次 `D:\Qt\6.11.1\msvc2022_64\bin\windeployqt6.exe --debug --qmldir client\qml client\build\shadow-worker-client.exe`。

### Go / CGO / whisper.cpp

9. **gcc 16 不兼容 Go cgo**：产生无效 COFF 头。必须用 w64devkit 1.21.0（gcc 13.2.0）。

10. **`go mod vendor` 会清掉 vendor 里的 patch**：whisper Go 绑定的 `whisper.go` 需要在 LDFLAGS 加 `-lgomp`（Windows OpenMP）。`build-whisper-cgo.bat` 每次构建前自动 patch。

11. **ggml 静态库命名**：CMake 产出 `ggml.a`（无 `lib` 前缀），cgo 的 `-lggml` 按 `libggml.a` 查找。构建后需重命名。

12. **MinGW 头文件缺 `THREAD_POWER_THROTTLING_*`**：whisper.cpp 的 `ggml-cpu.c` 需要 `#ifndef` fallback 定义。详见 `backend/WHISPER_BUILD.md`。

### gRPC / Protobuf

13. **Qt protobuf setter 命名**：Qt 生成的 stub 用 camelCase（`setMode`、`setFields`），不是 snake_case。map 类型用 `FieldsEntry` 嵌套类。

14. **Qt gRPC 回调签名**：`QGrpcCallReply::finished` 的回调接收 `const QGrpcStatus &status` 参数，不是 `reply->status()`。`reply->read<T>()` 返回 `std::optional<T>`。

15. **信号复用导致副作用**：`voiceClient.resultReady` 同时被录音浮窗和 Test Connection 监听。Test Connection 必须用独立信号 `connectionTested`，否则会误触发浮窗。**注意：`connectionTested` 的 `Connections` 必须放在 `main.qml` 的全局 Connections（和 `onResultReady` 同一个 target: voiceClient），不能放在 SettingsPage 里**——实测 SettingsPage 里的 Connections 收不到该信号（疑似 `import ShadowWorker` 的 `VoiceClient` 类型注册 + context property 冲突，详见坑 19）。

16. **ResultBubble 的 TextArea 必须单向绑定**：`TextArea { text: root.text }`，**不要加 `onTextEdited` 回写**。双向回写会形成循环——外部清空 `result` 时，TextArea 的 `onTextChanged→textEdited` 用内部旧值回写，导致第二次识别/close 后仍显示第一次的文字（坑 #2 的变体，在 `TextArea` 上同样成立）。

### 架构

17. **录音气泡 × 和结果气泡 × 职责不同**：录音气泡 × = 彻底关闭整个浮窗 + 设 `abandoned` 标记阻止后续 ASR 结果。结果气泡 × = 只关结果窗，pill 回到 `idle` 状态。

18. **热键 hold/press 模式**：
    - **press 模式**：`RegisterHotKey` toggle（按一次开始，再按一次停止）。
    - **hold 模式**：`RegisterHotKey` 检测按下 + `QTimer` 轮询 `GetAsyncKeyState` 检测松开。
    - **绝对不要用 `WH_KEYBOARD_LL` 低级键盘钩子**：其回调跑在系统键盘链路里，若回调里同步执行 emit→gRPC 等慢操作，会阻塞整个系统的键盘输入，导致**电脑卡死**。已废弃，改为轮询。
    - **`onPressed` 必须防重入**：`RegisterHotKey` 在某些配置下按住期间会重复触发 `WM_HOTKEY`（约每 30ms 一次）。若 `onPressed` 不挡，会反复 `startRealRecording` → 后端反复重建采集器 → 卡死。用 `recordingWindow.state !== "listening"` 同步判断（不要用 `voiceClient.recording`，它是 gRPC 异步回调后才置位，挡不住快速重复）。
    - **轮询定时器只能在未运行时启动**：首次按下 `if (!m_holdPollTimer.isActive()) m_holdPollTimer.start()`，否则重复的 WM_HOTKEY 会不断重置 50ms 定时器，tick 永不执行，松开检测不到。
    - **`stopHoldPolling()` 不要清零 `m_holdVk`**：`m_holdVk` 是注册时设的"配置"，清零会导致第二次按下时 tick 因 `m_holdVk==0` 立即退出。只在 `unregisterAll` 时清零。

19. **引擎热切换**：`asr.EngineHolder`（atomic.Pointer）在 `ConfigServer.SaveConfig` 后自动 `Rebuild`。重建失败保留旧引擎。

### 构建

20. **ninja 增量编译不会因单个 `.qml` 改动重打包 qrc**：`qt_add_qml_module` 把 QML 编译进 qrc，但 ninja 有时判定"no work to do"，导致改了 `.qml` 后运行的 exe 还是旧 QML（按钮 onClicked 不生效等）。**症状**：`ninja: no work to do`，但实际 QML 已改。**修复**：删 `build\.qt\rcc\*` + `build\ShadowWorker`（复制的 QML）+ 旧 exe，重新 `cmake -B build && cmake --build build` 强制重新生成 qrc。

21. **热键注册必须防抖 + 延迟到配置加载后**：`syncFromViewModel` 是 `Qt.callLater` 异步执行的，若在 `Component.onCompleted` 同步调 `registerRecordHotkey()`，此时 `recordMode` 还是默认值，会用错误的 hold/press 注册。注册应放在 `syncFromViewModel` 末尾，或用防抖 Timer（150ms）包裹，避免 ViewModel 多个 changed 信号密集触发导致反复 unregister/register。

### Vision / VLM

22. **VLM provider 字段数据流照抄 LLM，不要照抄 ASR**：VLM provider 字段集（name/baseUrl/model/apiKey/apiFormat/authType，无 language）与 LLM 完全一致，故 `SettingsPage.qml` 的 VLM 接通应镜像 `updateLlmFields`/`flushLlmFields`，**不要**镜像 ASR（ASR 多一个 language + SelectBox）。通用 helper `providerField(key, field, category)` / `activeProviderType(key, category)` 原先只认 `"asr"`/`"llm"`，接 VLM 时必须在这两个函数里加 `category === "vlm"` 分支读 `viewModel.vlmProviders`，否则字段永远读空。

23. **前端 `vlmMode` 字符串 ↔ 后端 `vlm_mode` 不一致**：前端 radio 用 `"scheduled"`/`"ondemand"`（无下划线），后端/proto `vlm_mode` 用 `"scheduled"`/`"on_demand"`（带下划线），关闭态是 `"off"`。必须在 `SettingsPage.qml` 用 `toBackendVlmMode(vlmMode, vlmEnabled)` / `fromBackendVlmMode(backendMode)` 双向映射，不能直接赋值。`vlmEnabled` 开关只在前端存在，后端通过 `vlm_mode="off"` 表示关闭。

24. **VLM Test Connection 探测用 1×1 PNG，不要用空字节**：后端 `voice_server.go` 的 `case "vlm"` 不能传空 `[]byte{}`（`Describe` 会因 `len(imagePNG)==0` 报"空图片"），必须合成一张最小的 1×1 RGBA PNG（`image.NewRGBA` + `png.Encode`，约 70 字节）。`Describe` 只校验 `len>0` 不校验尺寸，故足以验证整条 HTTP 链路（鉴权/路由/模型/响应解析）。需在 `vlm/engine.go` 加导出 helper `NewCloudEngineForTest(cfg config.VLMProvider)`（仿 `llm.NewCloudEngineForTest`），因为 `newCloudEngine` 是未导出的。

25. **Capture Range = "screen" 整屏截图用虚拟屏坐标**：多显示器整屏截图不是 `GetWindowRect(0)`（桌面窗口尺寸不准），而是 `GetSystemMetrics(SM_CXVIRTUALSCREEN/SM_CYVIRTUALSCREEN)` 取虚拟屏（所有显示器并集）宽高，配 `GetDC(0)`（覆盖整块虚拟屏的 DC）。虚拟屏原点可能为负（副屏在主屏左/上时），但 `GetDC(0)` 返回的 DC 已对齐到虚拟屏左上角，故 BitBlt 源坐标恒用 `(0,0)`。`CaptureWindowPNG` 的 BitBlt+GetDIBits+png.Encode 尾段已抽成公共 `capturePNGFromDC(srcDC, x, y, w, h)` + `bitsToPNG(bits, w, h)`，整屏和窗口捕获都复用，避免重复。

26. **`config.proto` 新增字段编号要查全**：`ConfigData` 消息字段编号到 `movement_display_idle_s=28`，新增 `vlm_capture_range` 必须用 **29**（不能想当然用 27，27 已被 `movement_input_idle_s` 占用）。改 proto 后必须重新生成 Go stub（`protoc -I proto --go_out=... --go-grpc_out=... proto/config.proto`），Qt stub 由 `cmake --build` 的 `qt_add_protobuf` 自动重生成。`gen_proto.bat` 只处理 `overview.proto`，config.proto 要手动跑 protoc。

27. **构建报 `LNK1168: 无法打开 xxx.dll 进行写入` = 客户端在运行**：链接阶段 DLL 写入失败（错误码 1168）几乎都是 `shadow-worker-client.exe` 正在运行，锁住了它加载的 `config_proto.dll` 等产物。`taskkill /F` 掉客户端进程后重新 `cmake --build` 即可。不是代码问题。

## 项目结构

```
shadow-worker/
├── backend/
│   ├── cmd/shadow-worker/main.go        # 入口
│   ├── internal/
│   │   ├── asr/                          # ASR 引擎（cloud.go / local.go / holder.go）
│   │   ├── audio/                        # Win32 waveIn + FFT + 频谱
│   │   ├── config/                       # YAML 配置
│   │   ├── grpcapi/                      # gRPC 服务实现
│   │   └── storage/                      # SQLite
│   ├── third_party/whisper.cpp/          # whisper 源码（含 MinGW 补丁）
│   ├── build-whisper-cgo.bat             # 一键构建（含 vendor patch）
│   ├── build-whisper-libs.bat            # 重建 whisper 静态库
│   └── WHISPER_BUILD.md                  # whisper 构建文档
├── client/
│   ├── src/
│   │   ├── asr/voiceclient.h/.cpp        # gRPC 客户端
│   │   ├── viewmodels/settings_vm.h/.cpp # 配置 ViewModel
│   │   ├── hotkey/globalhotkey.h/.cpp    # 全局热键（hold/press）
│   │   ├── audio/                        # 音频设备管理
│   │   └── ui/traycontroller.h/.cpp      # 系统托盘
│   ├── qml/
│   │   ├── main.qml                      # 主窗口 + 录音流程
│   │   ├── RecordingWindow.qml           # 双窗口录音浮窗
│   │   ├── pages/SettingsPage.qml        # 设置页（最大文件）
│   │   └── components/                   # TextField/Button/Card/Toast 等
│   └── build/                            # 编译产物 + DLL
├── proto/                                # .proto 定义
├── modules/                              # whisper .bin 模型
├── scripts/
│   ├── gen_proto.bat                     # proto 生成
│   └── build_client.bat                  # 客户端构建（含 vcvars）
└── ui-wireframes/ui-wireframe-v2.html    # UI 线框稿（设计参考）
```
