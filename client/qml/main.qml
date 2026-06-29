// main.qml - Shadow Worker main window (v2 rewrite)
// 180px sidebar (Overview/Timeline/Settings/System) + content view switch + global Toast.
// Source of truth: docs/ui-spec-v2.md section 2.
// All strings English; Chinese via Qt i18n (.ts/.qm).

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

    ApplicationWindow {
    id: mainWindow
    visible: !autostartMode
    width: 1200
    height: 720
    // Fixed size: no user resize (simplifies layout). DPI scaling handled by Qt6.
    minimumWidth: 1200
    maximumWidth: 1200
    minimumHeight: 720
    maximumHeight: 720
    // Frameless: we draw our own TitleBar (min + close).
    // Language switching now lives in SystemPage → Interface Language card.
    // title stays "Shadow Worker" (NOT qsTr) so singleinstance.cpp's
    // FindWindowW(nullptr, L"Shadow Worker") keeps working after i18n.
    flags: Qt.Window | Qt.FramelessWindowHint
    title: "Shadow Worker"

    // Close button (X) -> hide to tray, NOT quit. The app stays alive with the
    // tray icon present (QApplication::setQuitOnLastWindowClosed(false)).
    // Real exit only via the tray menu's Quit item.
    onClosing: function(close) {
        close.accepted = false
        mainWindow.hide()
    }

    // 录音浮窗（pillWindow/resultWindow）是 Qt.Tool 类型，属于主窗口的
    // transient 子窗口。当主窗口最小化时，Win32 会自动隐藏 Tool 窗口——
    // 即使浮窗的 visible=true。这违背"浮窗完全独立于主窗口"的需求。
    // 监听主窗口 visibility：最小化/隐藏后，若浮窗应该可见，强制重新显示。
    onVisibilityChanged: {
        if (mainWindow.visibility === Window.Minimized && recordingWindow.anyVisible) {
            Qt.callLater(refreshFloatingWindows)
        }
    }
    onVisibleChanged: {
        if (!mainWindow.visible && recordingWindow.anyVisible) {
            Qt.callLater(refreshFloatingWindows)
        }
    }
    function refreshFloatingWindows() {
        // 重新 show 被系统隐藏的浮窗（仅当它本应可见）
        if (recordingWindow.anyVisible) {
            recordingWindow.refreshVisibility()
        }
    }

    // tray menu / icon click handlers
    Connections {
        target: trayController
        function onShowMainRequested() {
            mainWindow.show()
            mainWindow.raise()
            mainWindow.requestActivate()
        }
        function onSettingsRequested() {
            currentView = "settings"
            mainWindow.show()
            mainWindow.raise()
            mainWindow.requestActivate()
        }
        // 托盘菜单"截图"：弹全屏框选覆盖层（与热键走同一路径）。
        function onScreenshotRequested() {
            startScreenshot()
        }
        // onQuitRequested handled in C++ (-> QApplication::quit), but we also
        // tear down any recording here for cleanliness.
        function onQuitRequested() {
            if (audioRecorder && audioRecorder.recording)
                audioRecorder.stopRecording()
        }
    }
    color: Theme.bg2

    property string currentView: "overview"

    // 截图防重入标志：press 模式热键按住期间 WM_HOTKEY 会重复触发（约 30ms
    // 一次），若不挡会反复弹出框选覆盖层。用本地同步 state 挡，截图完成/
    // 取消后清零（坑 #18 同款套路：不能用 gRPC 异步标志）。
    property bool screenshotInFlight: false

    // 弹出截图框选覆盖层。热键、托盘菜单、设置页"Capture Now"都走这里。
    // saveDir 由 screenshotController 内部用默认（%APPDATA%\shadow-worker
    // \screenshots，与后端 VLM 截图目录一致），QML 不需关心路径。
    function startScreenshot() {
        if (screenshotInFlight) return
        if (!screenshotController) return
        screenshotInFlight = true
        screenshotController.capture()
    }

    // 截图完成/取消回调。放在 target:screenshotController 的 Connections 里
    // （下方），用本地 state 触发。

    // ColumnLayout: TitleBar on top (fixed 36px) + RowLayout fills the rest.
    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        TitleBar {
            window: mainWindow
            Layout.fillWidth: true
        }

    RowLayout {
        Layout.fillWidth: true
        Layout.fillHeight: true
        spacing: 0

        // ==================== Sidebar ====================
        Rectangle {
            id: sidebar
            Layout.fillHeight: true
            Layout.preferredWidth: Theme.sidebarWidth
            color: Theme.bg2
            border.color: Theme.rule
            border.width: 0

            // right divider line
            Rectangle {
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: 1
                color: Theme.rule
            }

            ColumnLayout {
                anchors.fill: parent
                anchors.topMargin: 16
                spacing: 0

                Repeater {
                    model: [
                        { view: "overview", label: qsTr("Overview"), icon: "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z" },
                        { view: "timeline", label: qsTr("Timeline"), icon: "M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10 10-4.5 10-10S17.5 2 12 2zm4.2 14.2L11 13V7h2v5l4.5 2.7-.8 1.5z" },
                        { view: "settings", label: qsTr("Settings"), icon: "M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58a.49.49 0 0 0 .12-.61l-1.92-3.32a.488.488 0 0 0-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54a.484.484 0 0 0-.48-.41h-3.84a.484.484 0 0 0-.48.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96a.488.488 0 0 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.07.63-.07.94s.02.64.07.94l-2.03 1.58a.49.49 0 0 0-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.27.41.48.41h3.84c.24 0 .44-.17.48-.41l.36-2.54c.59-.24 1.13.57 1.62.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.03-1.58zM12 15.6A3.6 3.6 0 1 1 12 8.4a3.6 3.6 0 0 1 0 7.2z" },
                        { view: "system",   label: qsTr("System"),   icon: "M20 18c1.1 0 1.99-.9 1.99-2L22 5c0-1.1-.9-2-2-2H4c-1.1 0-2 .9-2 2v11c0 1.1.9 2 2 2H0v2h24v-2h-4zM4 5h16v11H4V5z" }
                    ]

                    delegate: Rectangle {
                        required property var modelData

                        Layout.fillWidth: true
                        Layout.preferredHeight: 44
                        color: currentView === modelData.view ? Theme.accentBg : "transparent"

                        // active left border 3px
                        Rectangle {
                            visible: currentView === modelData.view
                            anchors.left: parent.left
                            anchors.top: parent.top
                            anchors.bottom: parent.bottom
                            width: 3
                            color: Theme.accent
                        }

                        RowLayout {
                            anchors.fill: parent
                            anchors.leftMargin: 18
                            spacing: 10

                            // SVG icon (Image loads .svg file; swaps source for active/inactive)
                            Image {
                                Layout.preferredWidth: 18
                                Layout.preferredHeight: 18
                                sourceSize.width: 18
                                sourceSize.height: 18
                                source: currentView === modelData.view
                                        ? "qrc:/qt/qml/ShadowWorker/qml/icons/" + modelData.view + "_active.svg"
                                        : "qrc:/qt/qml/ShadowWorker/qml/icons/" + modelData.view + ".svg"
                            }

                            Text {
                                text: modelData.label
                                color: currentView === modelData.view ? Theme.accent : Theme.muted
                                font.pixelSize: Theme.fontBody
                            }
                        }

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: currentView = modelData.view
                        }
                    }
                }

                Item { Layout.fillHeight: true }
            }
        }

        // ==================== Content ====================
        Rectangle {
            Layout.fillWidth: true
            Layout.fillHeight: true
            color: Theme.bg2

            StackLayout {
                id: contentStack
                anchors.fill: parent
                currentIndex: ["overview", "timeline", "settings", "system"].indexOf(currentView)

                OverviewPage {
                    viewModel: overviewVm
                    // 跳到设置页并定位到"行为采集"tab（而非默认的音频输入）
                    onManageAppsRequested: {
                        currentView = "settings"
                        settingsPage.activeTab = "apps"
                    }
                }
                TimelinePage {
                    viewModel: timelineVm
                }
                SettingsPage {
                    id: settingsPage
                    viewModel: settingsVm
                    // whitelistVm 直接作为全局 context property 使用，
                    // 不通过传值注入（属性名与 context property 同名会导致绑定自引用）。
                }
                SystemPage {}
            }

            // global toast (top-right)
            Toast {
                id: globalToast
                anchors.top: parent.top
                anchors.right: parent.right
                anchors.topMargin: 16
                anchors.rightMargin: 16
            }
        }
    }
    }  // ColumnLayout

    // global toast helper for child pages
    // type: optional, "success" (default) | "error" | "warning"
    function toast(text, type) {
        globalToast.show(text, type)
    }

    // ================================================================
    // Recording window (HTML section 2: recording bubble + result bubble)
    // ================================================================
    // A standalone top-level Window in the same process: frameless,
    // stays-on-top, no taskbar entry. Positioned above the taskbar, centered.
    // Shows even when the main window is minimized to the tray.
    RecordingWindow {
        id: recordingWindow
    }

    // 截图 + VLM 分析结果窗口（独立置顶窗口，非依附主窗口）。
    // 由 screenshotController.finished 触发显示（仅开启 VLM 时），由
    // collectionClient.imageAnalyzed 信号回填摘要。
    ScreenshotResultWindow {
        id: screenshotResult
    }

    // ---- real recording flow driven by the global hotkey (or demo button) ----
    // globalHotkey.activatedWithName fires when the registered OS hotkey is
    // pressed. We start a REAL capture (not the demo state machine) and show
    // the recording window with its wave driven by live mic level.
    // ---- real recording flow: backend captures audio (waveIn), streams
    // 16-band FFT levels to us for the spectrum, and runs ASR on stop. ----
    function startRealRecording() {
        // backend opens the mic; the device id is read from QSettings by the
        // backend (we pass empty = default for now; device routing TBD).
        voiceClient.start("")
        // 设置当前使用的模型名（显示在结果气泡左下角）。
        // 显示 provider 的 model 字段（如 glm-asr-2512），不是 provider key。
        if (settingsVm) {
            if (settingsVm.asrMode === "local") {
                recordingWindow.asrModelName = settingsVm.asrLocalModelName || "local"
            } else {
                recordingWindow.asrModelName = activeProviderModel(settingsVm.asrProviders, settingsVm.asrActiveProvider)
            }
            // 润色模型名：只要配置了 LLM provider 就显示模型名，
            // 不受"自动润色"开关（llmEnabled）影响——后者只控制识别后是否
            // 自动触发，手动润色始终可用（与后端 llm.New 解耦 enabled 一致）。
            recordingWindow.polishModelName = activeProviderModel(settingsVm.llmProviders, settingsVm.llmActiveProvider)
        }
        recordingWindow.startRealRecording()
    }

    // 从 provider 列表里找激活 provider，返回它的 model 字段。
    // 用于结果气泡显示真实模型名（如 glm-asr-2512），而非 provider key。
    function activeProviderModel(providers, activeKey) {
        if (!providers) return ""
        for (var i = 0; i < providers.length; i++) {
            if (providers[i].key === activeKey) {
                return providers[i].model || activeKey
            }
        }
        return activeKey || ""
    }
    function stopRealRecording() {
        // backend stops capture + runs ASR; result arrives via onResultReady
        // 注入模式（injectMode=auto）：识别/润色过程中不弹结果气泡。
        recordingWindow.injecting = (settingsVm && settingsVm.llmInjectMode === "auto")
        recordingWindow.startTranscribing()
        voiceClient.stop()
    }

    // 尝试把文本注入当前焦点输入框（injectMode=auto 时调用）。
    // 成功返回 true（调用方据此收起气泡窗口）；失败返回 false（调用方降级弹气泡）。
    function tryInject(text) {
        if (!textInjector || !text) return false
        if (textInjector.inject(text)) {
            // 注入成功：收起录音窗口，不弹结果气泡。
            recordingWindow.hide()
            toast(qsTr("Injected"), "success")
            return true
        }
        // 注入失败：清除 injecting 标志，让结果气泡能显示（降级）。
        recordingWindow.injecting = false
        return false
    }

    // live spectrum frames from the backend -> recording window
    Connections {
        target: voiceClient
        function onLevelsReady(bands, rms) {
            recordingWindow.setBands(bands, rms)
        }
        function onResultReady(text, error) {
            // 用户已放弃（点了 ×），忽略后端返回的 ASR 结果
            if (recordingWindow.abandoned) return
            if (error && error !== "") {
                recordingWindow.applyTranscriptionError(error)
                return
            }
            // injectMode=auto 且非自动润色：直接注入 ASR 原文。
            // 自动润色时注入延后到 onPolishReady（注入润色后结果）。
            if (settingsVm && settingsVm.llmInjectMode === "auto"
                && !(settingsVm.llmEnabled)) {
                if (tryInject(text)) return
                // 注入失败 → 降级：弹气泡 + 降级提示
                recordingWindow.applyTranscription(text, true)
            } else {
                recordingWindow.applyTranscription(text)
            }
        }
        // test connection 结果也在这里处理（main.qml 已验证能收到 voiceClient 信号）
        function onConnectionTested(message, error) {
            if (error && error !== "") {
                toast(qsTr("Connection failed: ") + error, "error")
            } else {
                toast(qsTr("Connection OK: ") + message, "success")
            }
        }
        // 润色结果：转给 RecordingWindow 更新 result。
        // 用户已放弃则忽略（和 onResultReady 一致）。
        function onPolishReady(originalText, polishedText, error) {
            if (recordingWindow.abandoned) return
            // injectMode=auto 且开启自动润色：注入润色后结果（润色失败用原文兜底）。
            // 非自动润色或 preview 模式：正常走气泡显示。
            if (settingsVm && settingsVm.llmInjectMode === "auto"
                && settingsVm.llmEnabled) {
                var finalText = (error && error !== "") ? originalText : polishedText
                if (finalText && tryInject(finalText)) return
                // 注入失败 → 降级
                recordingWindow.applyPolishResult(originalText, polishedText, error, true)
                if (error && error !== "") {
                    toast(qsTr("Polish failed: ") + error, "warning")
                }
            } else {
                recordingWindow.applyPolishResult(originalText, polishedText, error)
                if (error && error !== "") {
                    toast(qsTr("Polish failed: ") + error, "warning")
                }
            }
        }
    }

    Connections {
        target: globalHotkey
        // press 模式：toggle（按一次开始，再按一次停止）
        function onActivatedWithName(name) {
            if (name === "record") {
                if (recordingWindow.state === "listening")
                    stopRealRecording()
                else
                    startRealRecording()
            } else if (name === "screenshot") {
                startScreenshot()
            }
        }
        // hold 模式：按下开始录音。防重入：用 recordingWindow.state 同步判断
        // （它在 startRealRecording 里立即变 "listening"）。RegisterHotKey 在
        // 按住期间可能重复触发 WM_HOTKEY（按键重复），若不挡会导致反复 start
        // → 后端反复重建采集器/频谱流，主线程过载卡顿。voiceClient.recording
        // 是 gRPC 异步回调后才置位，挡不住快速重复触发，故用本地 state。
        function onPressed(name) {
            if (name === "record" && recordingWindow.state !== "listening")
                startRealRecording()
        }
        // hold 模式：松开停止录音
        function onReleased(name) {
            if (name === "record") {
                if (recordingWindow.state === "listening")
                    stopRealRecording()
            }
        }
    }

    // 截图覆盖层完成/取消回调。finished 时落盘成功 + 已写剪贴板；
    // 若开启"截图后自动 VLM 分析"，把【用户框选的这张图】送去后端分析
    // （AnalyzeImage，不重新截图），并在独立结果窗口展示摘要。
    Connections {
        target: screenshotController
        function onFinished(path) {
            screenshotInFlight = false
            // 截图本身已落盘 + 写剪贴板，提示一下
            toast(qsTr("Screenshot saved"), "success")
            if (settingsVm && settingsVm.screenshotWithVlm && collectionClient) {
                // 弹结果窗口（analyzing 态），然后发起分析
                screenshotResult.imagePath = path
                screenshotResult.summary = ""
                screenshotResult.errorText = ""
                screenshotResult.analyzing = true
                screenshotResult.show()
                collectionClient.analyzeImage(path)
            }
        }
        function onCancelled() {
            screenshotInFlight = false
        }
    }

    // VLM 图片分析结果（截图后自动分析）。
    // 独立信号 imageAnalyzed，不复用 vlmSummaryReady（坑 #15：避免 error
    // 参数语义混淆）。imagePath 用于结果窗口缩略图，error 非空则显示失败。
    Connections {
        target: collectionClient
        function onImageAnalyzed(imagePath, summary, error) {
            if (!screenshotResult.visible) return
            screenshotResult.analyzing = false
            if (error && error !== "") {
                screenshotResult.errorText = error
            } else {
                screenshotResult.summary = summary
            }
        }
    }

    // ---- demo trigger button (now drives the real flow too) ----
    Rectangle {
        parent: mainWindow.contentItem
        anchors.bottom: mainWindow.contentItem.bottom
        anchors.left: mainWindow.contentItem.left
        anchors.bottomMargin: 24
        anchors.leftMargin: 24
        width: demoTxt.implicitWidth + 24
        height: 32
        radius: 16
        color: demoMa.containsMouse ? Theme.accentBg2 : Theme.bg3
        border.color: demoMa.containsMouse ? Theme.accent : Theme.rule
        border.width: 1
        z: 1000

        Text {
            id: demoTxt
            anchors.centerIn: parent
            text: recordingWindow.state === "hidden"
                  ? qsTr("▶ Record") : qsTr("■ Stop")
            color: demoMa.containsMouse ? Theme.ink : Theme.muted
            font.pixelSize: 12
        }
        MouseArea {
            id: demoMa
            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            hoverEnabled: true
            onClicked: {
                if (recordingWindow.state === "listening")
                    stopRealRecording()
                else
                    startRealRecording()
            }
        }
    }

    Component.onCompleted: {
        // pull overview once on startup
        if (overviewVm) overviewVm.refresh()
        // 首次拉取 timeline（此时所有 context property 已绑定，
        // dataChanged 信号能正确传到 QML；放在 setChannel 里太早，
        // gRPC 本地回调可能早于 QML 绑定建立，导致首次数据丢失）
        if (timelineVm) timelineVm.refresh()
        // Load config from backend; once loaded, register the record hotkey
        // from the SAVED value (settingsVm.hotkeyRecord is just the default
        // "F9" until load() completes). This is the single source of truth for
        // the startup hotkey — matches the ai-voice-tool pattern of registering
        // from config at startup.
        if (settingsVm) settingsVm.load()
    }

    // register hotkeys as soon as the saved config arrives. 用 unregisterByName
    // 定向注销（而非 unregisterAll），让 record / screenshot 两个热键互不影响：
    // 改 record 只注销 record，改 screenshot 只注销 screenshot。
    Connections {
        target: settingsVm
        function onHotkeyRecordChanged() {
            if (globalHotkey) {
                var sc = settingsVm.hotkeyRecord || "Ctrl+Shift+R"
                globalHotkey.unregisterByName("record")
                globalHotkey.registerShortcut(sc, "record")
            }
        }
        function onHotkeyScreenshotChanged() {
            if (globalHotkey) {
                var sc = settingsVm.hotkeyScreenshot
                globalHotkey.unregisterByName("screenshot")
                if (sc && sc.length > 0) {
                    // 截图用 press 模式（按一次截一张），避免 hold 的按住连截。
                    globalHotkey.registerShortcut(sc, "screenshot", "press")
                }
            }
        }
    }

    // 白名单变化（增删改后 whitelistVm.refresh → modelReset）时，刷新概览页的
    // 采集应用列表，保持首页与设置页一致，无需用户手动点 Refresh。
    Connections {
        target: whitelistVm
        function onModelReset() {
            if (overviewVm) overviewVm.refresh()
        }
    }
}
