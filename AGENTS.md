# AGENTS.md — AI Agent 构建指南

Shadow Worker 是一个本地行为跟踪桌面应用：Go 后端（gRPC + SQLite + whisper.cpp CGO）+ Qt/QML 前端。

## 构建命令

### 统一构建脚本（推荐入口）

```cmd
scripts\build.bat backend [clean]   :: Go 后端（whisper CGO）
scripts\build.bat client [clean]    :: Qt 客户端（Debug）
scripts\build.bat all [clean]       :: 后端 + 客户端
scripts\build.bat run [clean]       :: 构建全部 + 启动后端 + 客户端
scripts\build.bat package           :: Release 构建 + windeployqt + Inno Setup 安装包
scripts\build.bat clean             :: 清理所有构建产物
```

`build.bat` 内部已处理 vcvars64 + Qt + Go 环境初始化、版本号自动生成（`VERSION` 文件）、whisper CGO patch、windeployqt（含 `--qmldir`）。**所有构建/编译/打包任务都应通过此脚本执行**，不要手动拼 vcvars64 + cmake 命令（易踩坑 #41 的 cmd 吞输出 / PowerShell AMSI 崩溃问题）。

注意：`build.bat` 是 `.bat` 文件，在 PowerShell 终端里直接 `.\scripts\build.bat client` 即可运行（PowerShell 能正确执行 .bat）。

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

32. **debug 截图落盘开关（已拆分为两个独立开关）**：排查截图/VLM 识别问题时用 `debug.save_vlm_screenshots: true`（只存"真正送去 VLM 分析"的截图，文件名 `<时分秒>-<app>.png`，量小，推荐排查用）；排查 movement/帧差采集问题时用 `debug.save_motion_screenshots: true`（只存"帧差判定的活动窗口帧"，文件名 `<时分秒>-mv-<app>.png`，**Electron 类应用会高频落盘每秒数张，仅短时排查用**）。截图落盘到 `%APPDATA%\shadow-worker\screenshots\<日期>\`。旧的统一开关 `debug.save_screenshots` 已删除（曾导致两类截图混在一起，VLM 截图被帧差刷屏淹没，看不出"送去识别的图到底对不对"）。开关由 `main.go` 分别注入到 `cfg.VLM.SaveScreenshots`（经 VLMConfig）和 `cfg.Movement.SaveScreenshots`（经 PrecisionConfig 传给 collector）。

33. **【环境陷阱】PowerShell `Start-Process -RedirectStandardOutput` 会阻塞调用方**：在 bash 里用 `powershell.exe -Command "Start-Process ... -RedirectStandardOutput <path>"` 启动后台进程，PowerShell 会等待子进程的 stdout 管道关闭，导致命令"卡住"直到 5 分钟超时（但子进程其实已正常启动）。**改用工具的 `run_in_background: true` 直接跑 exe**（不重定向），或用 `cmd.exe /c start /B`。

34. **【环境陷阱】Windows GUI 程序无控制台时 stderr/stdout 不输出**：`shadow-worker-client.exe` 是 GUI 程序，没有 attach 的控制台时，`qDebug()` / `qWarning()` 的输出**不会**写到任何文件（即使 `run_in_background` 捕获的 stderr.log 也是空的）。排查 QML/Qt 运行时问题时，不要依赖 qDebug 日志——改用**可见副作用**诊断（如在 QML 临时把调试信息显示到某个 Text，或设 `visible: true` 的占位元素验证渲染路径）。后端 `shadow-worker.exe` 是控制台程序，stderr 正常输出。

35. **`gen_proto.bat` 只覆盖 `overview.proto`，collection.proto/voice.proto 要手动 protoc**：改了 `proto/collection.proto`（或 voice.proto）后，Go stub 不会自动重生。手动跑（参考 AGENTS.md 顶部"重新生成 proto"段落）。Qt 侧 stub 由 `qt_add_protobuf` 在 `cmake --build` 时自动重生（但注意坑 #20：ninja 可能 "no work to do"，需删 `build\.qt\rcc\*` 强制重打 qrc）。

36. **proto TimelineSegment 加字段后，Qt gRPC client 的 `.summary()` getter 要 Qt stub 重生才存在**：proto 加 `string summary = 7` 后，Go stub 手动 protoc 重生（坑 35），Qt stub `cmake --build` 自动重生。但 C++ ViewModel 里调 `seg.summary()` 前，必须确认 Qt stub 已重生（否则编译报 undefined）——构建日志里应有 `Generating QtProtobuf collection_proto sources`。若 getter 仍 undefined，删 `client\build\` 下 collection 相关产物强制重生。

37. **【数据陷阱】InsertEvent 必须带 TS 和 app 信息，否则时间窗查询永远查不到**：`ListEvents` 用 `WHERE ts >= start AND ts < end` 半开区间，**TS 零值（time.Time{} → toUnix 返回 0）的 event 永远查不到**。早期 `voice_server.go` 的 StopRecording 写 event 时漏了 `TS: time.Now().UTC()` 和 `ForegroundApp()`，产生大量 ts=0 的孤儿行（timeline events tab 显示空）。写 event 范式照抄 `asr_server.go`：带 TS + app.Path/app.Name。schema.go 的 migrate 已加 `DELETE FROM events WHERE ts = 0` 清理历史孤儿。

38. **ViewModel 首次数据加载的时序坑：refresh 不要放在 setChannel 里**：`main.cpp` 里 `timelineVm.setChannel(channel)`（第73行）在 `setContextProperty`（第95行）**之前**调用。若在 setChannel 里立即 refresh，gRPC 本地回调（<50ms）可能早于 QML 绑定建立，`dataChanged` 信号发完时 QML 还没监听 → 首次进入页面无数据。**正确做法：refresh 放在 `main.qml` 的 `Component.onCompleted` 里调**（此时所有 context property 已绑定）。overview_vm 的 setChannel 末尾调 refresh 能工作只是碰巧时序对，不要照抄。

39. **QML role 名不要用 `text`（与 QML 内置属性冲突）**：`QAbstractListModel::roleNames()` 里如果把某个 role 命名为 `"text"`，delegate 里用 `required property string text` 绑定会失败——值永远是空字符串。原因是 `text` 是 QML 内置属性名（Text 元素、很多组件都有），required property 的绑定机制会被内置属性接管，model 的 role 值传不进来。**症状**：DB/proto/C++ 层 `item.text` 都有值（可用 `QFile` 落盘确认，坑 #34），但 QML delegate 的 `text` 永远空。**修复**：role 名改成不冲突的名字（如 `"evText"`），delegate 的 `required property string evText` 同步改名。命名 role 时避开 QML 保留/通用属性名：`text`、`parent`、`data`、`children`、`x`/`y`/`width`/`height` 等。

40. **ListView delegate 根项的 `Layout.topMargin/bottomMargin` 不生效**：ListView 用 delegate 的 `implicitHeight` 来定位每行，而 Layout 系统的 margin 只在**被父 Layout 管理**时才参与高度计算——ListView 不是 Layout，所以 delegate 根（ColumnLayout/RowLayout）上的 `Layout.topMargin/bottomMargin` 被**静默忽略**。**症状**：改 margin 数值界面完全无变化。**修复**：行间距用 **`ListView.spacing`** 属性（这个 ListView 真正认）；delegate 内部的子项（被 ColumnLayout 管理的 RowLayout 等）仍可正常用 Layout margin 做行内呼吸。

41. **【构建陷阱】此环境的 cmd.exe 吞输出 + PowerShell AMSI 间歇崩溃**：在 AI agent 的非交互终端里，`cmd.exe /c "..."` 只打印 Windows banner 后立即 `[H[2J[3J` 清屏返回，**命令体的 stdout/stderr 和 `> log` 重定向全部失效**（但 exit code 正常）。`build_client.bat` 等含 `setlocal enabledelayedexpansion` + 在 `if()` 块里展开带 `(x86)` 路径的 bat 还会触发 `此时不应有 \Microsoft` 解析错误。PowerShell `-File` 加载脚本则可能 `AmsiUtils.ScanBuffer AccessViolation` 间歇崩溃（与脚本内容部分相关，不稳定）。**可靠构建方式**：用 `scripts\_build_qt_clean.bat`（纯 ASCII 注释、不用 setlocal/delayedexpansion、不在 if 块展开带括号路径），通过 `powershell -Command "cmd /c '...bat' 2>&1 | Select-Object -Last N"` 调用——PowerShell 编排 + cmd 子进程执行 vcvars64+cmake，输出能正常捕获。**改 .qml 后**必须删 `client/build/.qt/rcc` + `client/build/ShadowWorker` + `client/build/CMakeCache.txt` 强制 reconfigure 重打 qrc（坑 #20 的强化版：光删 rcc 不够，CMakeCache 还在会跳过 configure 导致 `qmake_ShadowWorker.qrc missing`）。

42. **Q_INVOKABLE 方法不参与 QML 声明式绑定，Q_PROPERTY + NOTIFY 才会自动刷新**：在 ViewModel 里用 `Q_INVOKABLE int activeDurationSec() const` 暴露统计值，QML 里写 `text: viewModel.activeDurationSec()`（带括号的方法调用），**绑定只在首次求值时执行一次**，之后底层 `m_items` 数据变了（`replaceAll` / `dataChanged`）也不会重新求值 → 统计数字永远停在初始值（如 Works 时长一直显示 0）。`Q_INVOKABLE` 是给 QML **命令式调用**用的（如 `refresh()`），不进 Qt 的属性绑定追踪系统。**修复**：改成 `Q_PROPERTY(int activeDurationSec READ ... NOTIFY activeDurationSecChanged)`，getter 读 model 的值，在数据更新处（`replaceAll` 之后）`emit activeDurationSecChanged()`；QML 绑定改成无括号的属性引用 `viewModel.activeDurationSec`。范式照抄 `windowStartTs`/`loading`。判断口诀：**QML 绑定里带 `()` 的要警惕——如果是需要随数据刷新的值，必须用 Q_PROPERTY + NOTIFY，不能用 Q_INVOKABLE**。

43. **slog 把 `time.Duration` 值智能格式化（不是整数）**：`slog.Info("...", "thresh", inputIdle/time.Millisecond)` 期望打印 `15000`（ms 数），但 slog 对 `time.Duration` 类型的值会用 `String()` 方法格式化成 `15µs` / `15s` 这类人类可读形式，而不是裸整数。`time.Duration` 底层是 `int64` 纳秒，slog 无法区分"你想看纳秒/微秒/毫秒数"和"这是个时长"。**症状**：日志里阈值类字段显示 `15µs` 而非 `15000`，干扰排查（曾让人误以为阈值配置成微秒级）。**修复**：日志里要打"数值"而非"时长"时，先转成 `int64`：`int64(inputIdle/time.Millisecond)`。仅当你确实想显示时长（如 `since_engaged`）时才直接传 Duration。

44. **【时区】`time.Parse("2006-01-02", ...)` 默认 UTC，导致按天查询跨午夜错位**：日期字符串如 `"2026-06-21"` 用 `time.Parse` 解析得到 `2026-06-21 00:00 **UTC**`，查询窗口 `[day, day+24h)` 实为 `[本地 08:00, 次日 08:00)`（UTC+8）。后果：凌晨 0-8 点的事件被算进前一天；熬夜跨午夜的工作不切日（卡在前一天）。单机部署（后端与客户端同机器、同时区）应改用 `time.ParseInLocation("2006-01-02", dateStr, time.Local)`。**同类出口要一起改**：`ListActivitySegmentsByDate` / `ListEventsByDate` / `TodayActivityMinutes` 等所有"按天"查询的 storage helper 都可能用 `UTC().Truncate(24h)` 切日，必须统一改成本地零点（封装 `startOfLocalDay(t)` helper：`time.Date(y,m,d,0,0,0,0,t.Location())`）。排查信号：时间轴窗口的起止刻度与本地作息明显错位 8 小时。

45. **【采集】帧差信号在 Electron/Chromium 应用空闲时持续误报 → active 段异常延长**：VS Code / ZCode / TRAE 等基于 Electron 的 IDE，即使无键鼠输入，GPU 合成层仍会因光标闪烁、代码补全动画、git 状态刷新、LSP 诊断高亮等产生画面变化，帧差比例（`FrameDiff`）周期性超阈值（`ChangeRatio`，medium=0.002）→ 每个这样的 tick `strong=true` → `lastEngaged` 持续刷新 → `inferState` 永远停在 `engaged`/`active`，到不了 `idle`。后果：一条覆盖数小时（曾见 7.5h）的 active/engaged 巨怪段，半透明压暗后视觉上是"灰白死区"。**这是已知设计缺陷**（离开检测的断段判据是 `state==idle`，但帧差让 state 永远到不了 idle，断段逻辑失效）。诊断方法：开 `log.level: debug`，grep `信号/状态翻转`，看 `input_idle_ms`（键鼠真实空闲，会很高如 81000ms）与 `reason=frame`（帧差误报）同时出现即实锤。**【已修复（离开检测部分）】**：离开检测的断段判据已从 `state==idle && Since(lastEngaged)>=阈值` 改为直接看 `inputIdleMs >= AwayThresholdS*1000`（真实键鼠空闲，`winapi.LastInputTick()`），彻底绕开被帧差污染的 `lastEngaged`（见坑 #49 同期修复）。注意：**帧差对 `state` 显示的污染仍然存在**——Electron 空闲时 state 仍会停在 engaged/active（视觉上"压暗的灰白死区"），但不再导致跨数小时的巨怪段（离开检测已不依赖 state）。断段 end 回填改用 `now - inputIdleMs`（最后一次真实输入时刻）而非 `lastEngaged`。**副作用**：看视频/被动观看超 `AwayThresholdS`（默认 10min）没碰键鼠会被断段，回来开新段、留小空档——对工作跟踪场景可接受。**不要简单删掉帧差**——它对 VLM on_demand 触发和"看视频/被动观看"识别是必要的。s2 键鼠计算已从 `if !strong` 分支提到每 tick 无条件计算（离开判据依赖它）。

46. **【VLM on_demand】失败不更新冷却 → 429 限流雪崩（已知缺陷，待重构）**：`onDemandLoop` 的逻辑是 Trigger 失败就 `continue`、**不更新 `lastCaptureUnix`**（注释意图"让下次能立即重试"）。在帧差高频触发（坑 #45）+ VLM API 返回 429 的场景下，这变成恶性循环：冷却因 `sinceLast` 持续增大永远判"通过" → 疯狂入队 → 疯狂 Trigger → 更多 429。**症状**：VLM 截图频率远超配置的 `on_demand_motion_gap_s`（配置 60s 实际 22s 一张），日志里 `冷却通过` 密集出现且 `since_last_s` 单调递增、`on-demand VLM 触发失败 err=429` 反复。**临时缓解**：关闭 VLM（`mode: off`）。**待重构方向**（已与用户确认）：① 把 `Trigger` 内部的"截图"和"调 API"拆成两阶段，截图阶段绑定 app+图，API 阶段失败重发原图（不重新截图，省 PrintWindow 开销且状态一致）；② 失败也更新冷却（尝试一次就更新，冷却语义改为"距上次尝试"而非"距上次成功"）；③ 429 单独熔断（检测到 429 进 5 分钟静默期）；④ 加最小触发间隔兜底（不管多少活跃信号，最少 N 秒触发一次）。

47. **【构建】`taskkill /F` 等带斜杠参数的命令在 bash 下被吞成路径**：bash 环境里直接调 `taskkill /F /PID 123`，`/F` 会被 bash 当成路径（`F:/`）解析导致参数错误。**改用 `cmd.exe //c "taskkill /F /IM shadow-worker-client.exe"`**（双斜杠 `//c` 让 bash 不转义，整个命令串交给 cmd.exe）。同理 `tasklist /FI` 等带斜杠参数的 Windows 命令在 bash 下都要套 `cmd.exe //c "..."`。另外：bash 里 `... 2>nul` 的 `2>nul` 重定向会被 bash 当成文件名 `nul`，产生垃圾文件 `./nul`、`./backend/nul`——**重定向要整体包在 `cmd.exe //c "..."` 里**，或事后 `rm -f nul backend/nul` 清理。

48. **【VLM】PrintWindow 对"一闪而过的弹窗"返回陈旧缓存 → app 标签与截图内容不一致**：`Trigger` 里 `app = ForegroundApp()` 和 `CaptureWindowPNG(app.HWND)` 之间有时间差（中间有 DB 白名单查询往返）。若该期间一个弹窗（微信通知、输入法候选框）短暂抢前台又被释放，`ForegroundApp` 记下的是弹窗 app，但弹窗消失后 `PrintWindow(弹窗.HWND)` 拿到的是该窗口**被抢占前最后一帧合成缓存**（弹窗已不渲染，画的是它出现前的画面）。后果：事件记 app=微信，但截图内容是 Shadow Worker 客户端（弹窗前的画面）。**症状**：工作日志里 app 名与 VLM 摘要描述明显不符（app=Weixin 但摘要说"查看时间线"）。这是坑 #46 待重构的一部分（拆分截图/API 阶段、截图前校验前台一致性可缓解）。诊断：开 `debug.save_vlm_screenshots`，比对文件名 app 与截图内容。

49. **【时间轴】跨天段不裁剪 → 巨怪段把全天可视窗口撑爆，表现为"当天空白无记录"**：坑 #45 的 46h 巨怪段（`id=518`：6-22 23:37→6-24 22:19）横跨整个 6-23。`ListActivitySegments` 用区间重叠判据（`start_ts<dayEnd AND end_ts>dayStart`，`activity.go`），跨天段会**完整命中且不截断**——6-22/6-23/6-24 三天查询都返回同一条 46h 段。接着 `computeTimelineWindow`（`collection_server.go`）用这条段的 start/end 算可视窗口 → 窗口被撑到 ~48h；前端 `TimelineTrack.tsToX` 把窗口外的段 clamp 到边界、`hourTicks` 在 `>12h` 切 2h 步进 → 23 号的真实时刻只占窗口右半小段、刻度稀疏、一条灰色超长条压在最底层 → **视觉上就是"空白无记录"**（实际有一条横贯全天的段被压扁了）。**修复（治标，已做）**：新增 `clipSegmentsToDay(segs, dayStart, dayEnd)`，在 `QueryTimeline` 的 `aggregateSegments` 之后把跨天段按本地午夜虚拟切分（只动返回值不落库），各天只显示自己当天应有的部分；`computeTimelineWindow`/`LatestVLMSummary` 回填都基于裁剪后数据，窗口不再被撑爆。`TodayActivityMinutes`（`activity.go`）的 SQL 同步修：旧判据 `start_ts>=? AND end_ts<=?` 会漏掉跨天段（start<今天0点 或 end>今天24点 都不命中）→ 今日统计偏小；改用区间重叠判据 + 按天 clamp `MIN(end_ts,dayEnd)-MAX(start_ts,dayStart)`，只算当天内部分不重复计。**配套治本**：坑 #45 的离开检测已改用真实键鼠空闲（`inputIdleMs`），从源头堵住巨怪段。诊断：开 `log.level: debug` grep `离开断段`，看 `input_idle_ms` 是否 ≥ `away_thresh_ms`。

50. **【打包/单例】单例保护必须用 Named Mutex + `Local\` 前缀**：`CreateMutexW` + `GetLastError() == ERROR_ALREADY_EXISTS` 是 Windows 桌面单例事实标准（VSCode/Chrome/Discord 同方案），原子操作无竞态、进程崩溃内核自动释放。**必须用 `Local\` 前缀**（非 `Global\`）：`Global\` 需要 `SeCreateGlobalPrivilege`，非管理员用户创建会返回 `ERROR_ACCESS_DENIED`。Shadow Worker 是用户级桌面应用（客户端 `QProcess::startDetached` 拉起后端，继承当前用户权限），用 `Local\` 即可——每个会话独立，不同 RDP 用户各跑一份。后端 mutex 名 `Local\Shadow-Worker-Backend`，客户端 `Shadow-Worker-Client`（默认 Local 命名空间）。MCP 模式（`runMCPServer`）**不做单例**——短生命周期子进程，agent 每次调用起一个。客户端唤醒已有实例用 `FindWindowW(nullptr, L"Shadow Worker")` 按窗口标题匹配——**已知限制**：`qsTr("Shadow Worker")` 接入 `.qm` 翻译后标题会变，`FindWindow` 会失效。当前无 `.qm` 文件安全；后续如需国际化应改用窗口类名或 `RegisterWindowMessage` 方案。

51. **【后端】`signal.NotifyContext` 优雅退出必须配套不带 `/F` 的 taskkill**：Go 后端 `main.go` 的 `runBackgroundService` 末尾用 `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` + goroutine 跑 `grpcServer.Serve` + `select { case <-ctx.Done(): grpcServer.GracefulStop() }` 实现优雅退出。**关键**：只有不带 `/F` 的 `taskkill /PID` 才触发信号（发 `WM_CLOSE`/`CTRL_BREAK_EVENT`）→ `GracefulStop` → `defer`（`db.Close`、`coll.Stop` 等）正常执行。`taskkill /F` 是强杀，不发信号，`defer` 不保证执行。客户端 `BackendLauncher::stop()` 先发 `taskkill /PID`（graceful），等 2.5 秒，仍存活再 `taskkill /F /T /PID` 兜底——两阶段关闭让后端有机会走 graceful 路径。如果直接 `/F`，坑 50 的信号处理完全无效。

52. **【客户端】拉起后端用 `QProcess::startDetached` + 记 PID，不用 `start`**：`QProcess::start` 绑定父进程生命周期——父（客户端）退出子（后端）跟着死。但我们就是要主动管理后端生命周期（客户端退出时关后端），所以用 `startDetached` + 记 `m_pid` + `aboutToQuit` 时 `taskkill`。`startDetached` 异步，后端启动有 1-3s 延迟，客户端优雅降级（UI 显示 gRPC error 直到后端起来）。`stop()` 的两阶段关闭见坑 51。`QThread::msleep(2500)` 在 `aboutToQuit` 中会阻塞客户端退出 2.5s——可接受，用户点 Quit 后短暂延迟比后端脏退好。`/T` 连带子进程（后端实际无子进程，但是好习惯）。

53. **【打包】`package.bat` 后端构建必须调 `build-whisper-cgo.bat` 且输出路径作为命令行参数传入**：普通 `go build -ldflags="-s -w"` 在 whisper.cpp CGO 处失败（坑 9/10/11：gcc 16 不兼容、vendor patch 被 `go mod vendor` 清掉、ggml 静态库命名）。`build-whisper-cgo.bat` 自动 patch vendor + 设 CGO 环境 + 调 gcc 13.2.0。**致命细节**：该脚本第 60 行 `set "OUT=%~1"` 取的是**第一个命令行参数**，会覆盖预设的 `OUT` 环境变量；不传参时走默认 `..\build\shadow-worker.exe`。预设 `set "OUT=dist\bin\..."` 再 `call build-whisper-cgo.bat` 会得到空 `OUT` → exe 产出到 `build\` 而非 `dist\bin\` → windeployqt 和 iss 全断链。**必须** `call build-whisper-cgo.bat "路径"` 把输出路径作为参数传入。另：`windeployqt` 必须加 `--qmldir client\qml`，否则 QML 插件 DLL 可能漏部署（坑 8 同类问题）。`backend\config.yaml` 不存在（配置走 `%APPDATA%`），`package.bat` 不应拷贝 config，`.iss` 也不应有 config Source 行。

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
