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

28. **【重要，坑 25 的修正】GetDC+BitBlt 对硬件加速窗口（Electron/CEF）只能截到加载态/空白**：VS Code、ZCode、TRAE 等基于 Chromium/Electron 的 IDE 用 GPU 合成渲染，`GetDC(hwnd)+BitBlt` 拿到的是窗口框架 DC，内容是空白或启动加载画面（症状：截图文件恒定 13195 字节、VLM 永远回答"用户正在等待页面加载"）。**必须用 `PrintWindow(hwnd, memDC, PW_RENDERFULLCONTENT=2)`**（Windows 8.1+），它让窗口把合成后的真实内容绘制到我们提供的 DC。`winapi.PrintWindow` + `PW_RENDERFULLCONTENT` 常量已加在 `user32.go`。`CaptureWindow`（movement 帧差）和 `CaptureWindowPNG`（VLM 截图）都已改用此方式，整屏截图 `CaptureScreenPNG` 不受影响（GetDC(0) 是屏幕 DC）。排查截图问题时开 `debug.save_screenshots` 落盘看原图（见坑 32）。

29. **【致命】modernc/sqlite 不允许把 NULL 列扫进 Go string**（与 mattn 驱动行为不同）：`ADD COLUMN` 新加的列，旧行默认 NULL；用 `&seg.Summary`（string）扫会报 `sql: Scan error on column index N: converting NULL to string is unsupported`，导致整个 `ListActivitySegments` 失败 → QueryTimeline 返回 gRPC error → 前端 timeline 全空（但前端只显示"No activity"空态，掩盖了真实 error）。**必须用 `sql.NullString` 扫可空列，再取 `.String`**。`scanActivitySegment` 和 `LatestVLMSummary` 都已改用 NullString。教训：新增可空列后，所有 Scan 该列的地方都要 NullString 兜底；前端应渲染 `viewModel.error`（timeline_vm 有 error 属性但 TimelinePage 一直没显示，导致 error 被吞）。

30. **QVariantList + emit dataChanged 会让 Repeater 全量销毁重建所有 delegate**：ViewModel 用 `QVariantList m_segments` 存数据，refresh 时 `clear()`+重建+`emit dataChanged()`，QML 的 `Repeater { model: filteredSegments() }` 把返回值当"全新数组"→ 销毁并重建全部 delegate（269 个 × 布局重算 = 明显卡顿，30s 轮询时尤其明显）。**改用 `QAbstractListModel` 子类 + diff 增量更新**：`replaceAll` 用复合 key（如 startTs+appName）匹配旧行，结构一致时只对变化行发 `dataChanged(roles)`，未变的 delegate 不重建；结构剧变（换日期）才 `beginResetModel/endResetModel`。范式照抄 `whitelist_vm.h/.cpp`（Role 枚举 + Q_ENUM + rowCount/data/roleNames override）。delegate 从 `modelData.xxx` 改为 `required property <type> <role>`（Qt6 推荐范式）。过滤/倒序逻辑下移到 C++（Model 内置 filter 属性），QML 删掉所有 `slice/push/reverse` JS 数组操作。

31. **段聚合的语义：同一应用连续工作应合并为一段，只有切换应用才开新段**：`updateSegment` 的开新段判据是**"仅 app 变化"**（`c.curApp.Path != app.Path`），engaged/active/idle 之间的翻转都在段内滚动（idle 只是"在这件事上暂时思考"，不算离开）。早期版本用 `c.curState != state` 当判据，导致每秒的 state 翻转切碎成"每分钟一条"碎片。此外查询层 `aggregateSegments` 会把历史 DB 里已有的同 app 碎片段合并（兼容旧数据），聚合段无单一 DB ID，summary 回填只更新返回值不落库。`UpdateActivitySegmentEndTSAndState` 用于段内滚动更新 end_ts + state（一次 UPDATE 省 IO）。

32. **debug 截图落盘开关**：排查截图/VLM 识别问题时，在 config.yaml 加 `debug.save_screenshots: true`，所有截图（VLM 截图 `<时分秒>-<app>.png` + movement 帧 `<时分秒>-mv-<app>.png`）会落盘到 `%APPDATA%\shadow-worker\screenshots\<日期>\` 供分析。默认关闭（不落盘，截图只在内存流转给识别引擎）。开关由 `main.go` 从 `cfg.Debug.SaveScreenshots` 注入到 `cfg.VLM.SaveScreenshots` 和 `cfg.Movement.SaveScreenshots`（后者经 `PrecisionConfig` 传给 collector）。movement 只在帧差超阈值时保存（避免每 300ms 落盘刷屏）。

33. **【环境陷阱】PowerShell `Start-Process -RedirectStandardOutput` 会阻塞调用方**：在 bash 里用 `powershell.exe -Command "Start-Process ... -RedirectStandardOutput <path>"` 启动后台进程，PowerShell 会等待子进程的 stdout 管道关闭，导致命令"卡住"直到 5 分钟超时（但子进程其实已正常启动）。**改用工具的 `run_in_background: true` 直接跑 exe**（不重定向），或用 `cmd.exe /c start /B`。

34. **【环境陷阱】Windows GUI 程序无控制台时 stderr/stdout 不输出**：`shadow-worker-client.exe` 是 GUI 程序，没有 attach 的控制台时，`qDebug()` / `qWarning()` 的输出**不会**写到任何文件（即使 `run_in_background` 捕获的 stderr.log 也是空的）。排查 QML/Qt 运行时问题时，不要依赖 qDebug 日志——改用**可见副作用**诊断（如在 QML 临时把调试信息显示到某个 Text，或设 `visible: true` 的占位元素验证渲染路径）。后端 `shadow-worker.exe` 是控制台程序，stderr 正常输出。

35. **`gen_proto.bat` 只覆盖 `overview.proto`，collection.proto/voice.proto 要手动 protoc**：改了 `proto/collection.proto`（或 voice.proto）后，Go stub 不会自动重生。手动跑（参考 AGENTS.md 顶部"重新生成 proto"段落）。Qt 侧 stub 由 `qt_add_protobuf` 在 `cmake --build` 时自动重生（但注意坑 #20：ninja 可能 "no work to do"，需删 `build\.qt\rcc\*` 强制重打 qrc）。

36. **proto TimelineSegment 加字段后，Qt gRPC client 的 `.summary()` getter 要 Qt stub 重生才存在**：proto 加 `string summary = 7` 后，Go stub 手动 protoc 重生（坑 35），Qt stub `cmake --build` 自动重生。但 C++ ViewModel 里调 `seg.summary()` 前，必须确认 Qt stub 已重生（否则编译报 undefined）——构建日志里应有 `Generating QtProtobuf collection_proto sources`。若 getter 仍 undefined，删 `client\build\` 下 collection 相关产物强制重生。

37. **【数据陷阱】InsertEvent 必须带 TS 和 app 信息，否则时间窗查询永远查不到**：`ListEvents` 用 `WHERE ts >= start AND ts < end` 半开区间，**TS 零值（time.Time{} → toUnix 返回 0）的 event 永远查不到**。早期 `voice_server.go` 的 StopRecording 写 event 时漏了 `TS: time.Now().UTC()` 和 `ForegroundApp()`，产生大量 ts=0 的孤儿行（timeline events tab 显示空）。写 event 范式照抄 `asr_server.go`：带 TS + app.Path/app.Name。schema.go 的 migrate 已加 `DELETE FROM events WHERE ts = 0` 清理历史孤儿。

38. **ViewModel 首次数据加载的时序坑：refresh 不要放在 setChannel 里**：`main.cpp` 里 `timelineVm.setChannel(channel)`（第73行）在 `setContextProperty`（第95行）**之前**调用。若在 setChannel 里立即 refresh，gRPC 本地回调（<50ms）可能早于 QML 绑定建立，`dataChanged` 信号发完时 QML 还没监听 → 首次进入页面无数据。**正确做法：refresh 放在 `main.qml` 的 `Component.onCompleted` 里调**（此时所有 context property 已绑定）。overview_vm 的 setChannel 末尾调 refresh 能工作只是碰巧时序对，不要照抄。

39. **QML role 名不要用 `text`（与 QML 内置属性冲突）**：`QAbstractListModel::roleNames()` 里如果把某个 role 命名为 `"text"`，delegate 里用 `required property string text` 绑定会失败——值永远是空字符串。原因是 `text` 是 QML 内置属性名（Text 元素、很多组件都有），required property 的绑定机制会被内置属性接管，model 的 role 值传不进来。**症状**：DB/proto/C++ 层 `item.text` 都有值（可用 `QFile` 落盘确认，坑 #34），但 QML delegate 的 `text` 永远空。**修复**：role 名改成不冲突的名字（如 `"evText"`），delegate 的 `required property string evText` 同步改名。命名 role 时避开 QML 保留/通用属性名：`text`、`parent`、`data`、`children`、`x`/`y`/`width`/`height` 等。

40. **ListView delegate 根项的 `Layout.topMargin/bottomMargin` 不生效**：ListView 用 delegate 的 `implicitHeight` 来定位每行，而 Layout 系统的 margin 只在**被父 Layout 管理**时才参与高度计算——ListView 不是 Layout，所以 delegate 根（ColumnLayout/RowLayout）上的 `Layout.topMargin/bottomMargin` 被**静默忽略**。**症状**：改 margin 数值界面完全无变化。**修复**：行间距用 **`ListView.spacing`** 属性（这个 ListView 真正认）；delegate 内部的子项（被 ColumnLayout 管理的 RowLayout 等）仍可正常用 Layout margin 做行内呼吸。

41. **【构建陷阱】此环境的 cmd.exe 吞输出 + PowerShell AMSI 间歇崩溃**：在 AI agent 的非交互终端里，`cmd.exe /c "..."` 只打印 Windows banner 后立即 `[H[2J[3J` 清屏返回，**命令体的 stdout/stderr 和 `> log` 重定向全部失效**（但 exit code 正常）。`build_client.bat` 等含 `setlocal enabledelayedexpansion` + 在 `if()` 块里展开带 `(x86)` 路径的 bat 还会触发 `此时不应有 \Microsoft` 解析错误。PowerShell `-File` 加载脚本则可能 `AmsiUtils.ScanBuffer AccessViolation` 间歇崩溃（与脚本内容部分相关，不稳定）。**可靠构建方式**：用 `scripts\_build_qt_clean.bat`（纯 ASCII 注释、不用 setlocal/delayedexpansion、不在 if 块展开带括号路径），通过 `powershell -Command "cmd /c '...bat' 2>&1 | Select-Object -Last N"` 调用——PowerShell 编排 + cmd 子进程执行 vcvars64+cmake，输出能正常捕获。**改 .qml 后**必须删 `client/build/.qt/rcc` + `client/build/ShadowWorker` + `client/build/CMakeCache.txt` 强制 reconfigure 重打 qrc（坑 #20 的强化版：光删 rcc 不够，CMakeCache 还在会跳过 configure 导致 `qmake_ShadowWorker.qrc missing`）。

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
