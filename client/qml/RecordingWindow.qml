// RecordingWindow.qml - standalone top-level windows hosting the recording
// bubble + result bubble (HTML section 2).
//
// ARCHITECTURE: TWO independent top-level Windows in the same process.
//   - pillWindow  : the recording pill. Fixed size + position. NEVER changes
//                   geometry, so it never jitters.
//   - resultWindow: the result bubble. Positioned just above the pill window.
//                   When the user resizes its textarea, ONLY resultWindow's
//                   geometry changes — the pill window is untouched.
//
// Splitting into two windows is the fundamental fix for the jitter: the pill
// window's geometry is completely independent of the result bubble's, so
// resizing the bubble cannot possibly move or flash the pill.
//
// Both windows: Frameless + StaysOnTop + Tool. The whole thing is driven by a
// single state machine below.

import QtQuick
import QtQuick.Window
import ShadowWorker

Item {
    id: root

    // ---- state machine ----
    property string state: "hidden"
    property string transcript: ""
    property string result: ""
    // 自动润色开关：读配置里的 LLM 启用状态。settingsVm 是全局 context property。
    // 开启时，ASR 识别出文字后自动调 voiceClient.polish() 润色。
    property bool autoPolish: settingsVm ? settingsVm.llmEnabled : false
    property bool resultPolishing: false
    // abandoned=true 表示用户点了 × 放弃，后续的 ASR 结果应被忽略
    property bool abandoned: false
    // 当前使用的模型名（显示在结果气泡右下角）
    property string asrModelName: ""
    property string polishModelName: ""
    // live 16-band spectrum from the backend (empty when idle)
    property var bands: []
    property int rmsLevel: 0

    readonly property string demoTranscript:
        "Please organize this requirements doc into structured meeting minutes, " +
        "then I will add some details..."
    readonly property string demoResult:
        "Please organize this requirements doc into structured meeting minutes."

    // expose so main.qml can drive the demo
    visible: true

    // ---- positioning: pill window bottom-center of the work area ----
    property real pillBottomEdgeY: 0
    property int gapAboveTaskbar: 24
    property int bubbleGap: 10   // gap between result bubble and pill
    property bool draggingPill: false

    function placePill() {
        // position the pill window bottom-center (one-shot, imperative — the pill
        // has no x/y binding; the OS drag also sets them directly).
        // resultWindow.x/y are DECLARATIVE bindings that read pillWindow.x/y,
        // so they follow automatically. We must NOT command-assign resultWindow
        // here, or its bindings break and y stops tracking height changes.
        var wa = windowHelper.primaryAvailableGeometry()
        if (!wa || !wa.width || wa.width <= 0) {
            pillPosRetry.restart()
            return
        }
        pillWindow.x = Math.round(wa.x + (wa.width - pillWindow.width) / 2)
        pillWindow.y = Math.round(wa.y + wa.height - gapAboveTaskbar - pillWindow.height)
        pillBottomEdgeY = pillWindow.y + pillWindow.height
        pillWindow.positioned = true
    }
    Timer {
        id: pillPosRetry
        interval: 50; repeat: false
        onTriggered: root.placePill()
    }

    // NOTE: resultWindow.x/y are DECLARATIVE BINDINGS (see the Window below).
    // They read pillWindow.x/y + own height. Never command-assign them.

    // ---- visibility ----
    readonly property bool anyVisible: state !== "hidden"

    // ---- state machine functions ----
    function show() {
        state = "listening"
        transcript = ""
        result = ""
        resultPolishing = false
        placePill()
    }
    function hide() {
        state = "hidden"
        // 清空上次的数据，下次打开时是干净的（避免残留旧文字）
        transcript = ""
        result = ""
        resultPolishing = false
    }
    // close = ABANDON at any state: discard recording/partial result, just hide.
    // Distinct from stopRealRecording (which transcribes + polishes).
    function close() {
        finishTimer.stop()
        resultPolishing = false
        // stop backend capture (result is ignored on abandon)
        if (voiceClient && voiceClient.recording) {
            voiceClient.stop()
        }
        hide()
    }
    function advance() {
        switch (state) {
            case "listening":
                transcript = demoTranscript
                state = "transcribing"
                break
            case "transcribing":
                state = "polishing"
                if (autoPolish && voiceClient && result.length > 0) {
                    result = demoResult
                    resultPolishing = true
                    voiceClient.polish(result)
                }
                break
            case "polishing":
                result = demoResult
                resultPolishing = false
                state = "completed"
                break
            case "completed":
                hide()
                break
        }
    }

    // ---- REAL recording flow (hotkey / demo button) ----
    // These differ from the demo advance(): they're driven by actual mic
    // capture. The listening state shows the wave driven by audioRecorder.level
    // (see RecordingBubble). finishRecording runs the transcribe->polish->done
    // pipeline with placeholder text (ASR not wired yet).
    property real recordStartMs: 0
    function startRealRecording() {
        recordStartMs = Date.now()
        abandoned = false
        state = "listening"
        transcript = ""
        result = ""
        resultPolishing = false
        bands = []
        rmsLevel = 0
        placePill()
    }
    // live spectrum frame from the backend (16 floats + rms 0..100)
    function setBands(b, rms) {
        bands = b
        rmsLevel = rms
    }
    function finishRecording() {
        // duration of the captured audio
        var secs = Math.max(1, Math.round((Date.now() - recordStartMs) / 1000))
        // placeholder result text — ASR/transcription is not wired yet.
        result = qsTr("Recording captured (%1s). Connect an ASR provider to transcribe.").arg(secs)
        // brief transcribe then polish then done
        state = "transcribing"
        finishTimer.restart()
    }
    // ---- cloud ASR transcription flow ----
    // Called right after stopRealRecording sends PCM to the backend. Enters
    // the transcribing state and waits for applyTranscription() (from the
    // asrClient.resultReady signal) or applyTranscriptionError().
    function startTranscribing() {
        result = ""
        resultPolishing = false
        state = "transcribing"
        // 清空频谱数据：转写期间不再有 live 数据，清空后波形回退到装饰性
        // 脉动动画（waveAnim），避免冻结在最后一帧的画面上。
        bands = []
        rmsLevel = 0
        // no timer here — the result arrives async via the gRPC stream
    }
    // backend returned recognized text -> 若开启自动润色则调 LLM，否则直接完成。
    function applyTranscription(text) {
        result = text
        if (autoPolish && voiceClient) {
            state = "polishing"
            resultPolishing = true
            voiceClient.polish(text)
        } else {
            state = "completed"
        }
    }
    // 润色结果回调（由 main.qml 的 onPolishReady 转发）。
    // 成功：替换 result 为润色文字；失败：保留原文 + 提示。
    function applyPolishResult(originalText, polishedText, error) {
        if (error && error.length > 0) {
            // 润色失败：保留原文，结果气泡仍显示 ASR 文字
            result = originalText
        } else {
            result = polishedText
        }
        resultPolishing = false
        state = "completed"
    }
    // backend reported an ASR error -> pill 显示红色叉叉 error 状态，
    // 结果气泡显示错误信息（不走 polish）。
    function applyTranscriptionError(errorMsg) {
        result = qsTr("Transcription failed: %1").arg(errorMsg)
        resultPolishing = false
        state = "error"
    }
    // finishTimer 保留用于 demo 流程（finishRecording）；真实录音不走它。
    Timer {
        id: finishTimer
        interval: 600
        repeat: false
        onTriggered: {
            state = "polishing"
            if (autoPolish && voiceClient && result.length > 0) {
                resultPolishing = true
                voiceClient.polish(result)
            }
        }
    }

    // ================================================================
    // PILL WINDOW (recording bubble) — fixed geometry, never jitters.
    // ================================================================
    Window {
        id: pillWindow
        visible: root.anyVisible
        flags: Qt.FramelessWindowHint | Qt.WindowStaysOnTopHint | Qt.Tool
        color: "transparent"
        width: 320
        height: 44
        property bool positioned: false

        RecordingBubble {
            anchors.fill: parent
            z: 1   // above the drag MouseArea (z:0) so the close button works
            state: root.state
            transcript: root.transcript
            // live 16-band spectrum from the backend drives the wave heights
            bands: root.bands
            onCloseRequested: function() {
                // 录音气泡 × = 彻底关闭整个浮窗（pill + result），放弃一切。
                // 标记 abandoned，后续 ASR 结果会被忽略（不会触发浮窗/润色）。
                root.abandoned = true
                if (voiceClient && voiceClient.recording) {
                    voiceClient.stop()
                }
                polishTimer.stop()
                finishTimer.stop()
                root.hide()
            }
        }

        // drag the pill window. resultWindow follows via its x/y bindings
        // (they read pillWindow.x/y), so we don't reposition it manually.
        // z:0 keeps this BELOW the RecordingBubble so its close button (and
        // other interactive children) receive clicks first; only empty areas
        // fall through to here and start a drag.
        MouseArea {
            anchors.fill: parent
            z: 0
            onPressed: function(mouse) {
                root.draggingPill = true
                windowHelper.startDrag(pillWindow)
            }
            onReleased: function(mouse) {
                root.draggingPill = false
                root.pillBottomEdgeY = pillWindow.y + pillWindow.height
            }
        }

        onVisibleChanged: if (visible && !positioned) root.placePill()
    }

    // ================================================================
    // RESULT WINDOW (result bubble) — sits above the pill, may resize freely.
    // The pill window is never touched when this resizes.
    // Both height and y are DECLARATIVE bindings so QML's property system keeps
    // them consistent in one evaluation pass — no imperative setY, no jitter.
    // ================================================================
    Window {
        id: resultWindow
        visible: root.anyVisible && root.state !== "listening" && root.state !== "idle"
        flags: Qt.FramelessWindowHint | Qt.WindowStaysOnTopHint | Qt.Tool
        color: "transparent"
        width: 320
        // height tracks the bubble's content height (header + textarea + actions)
        height: resultContent.implicitHeight
        // x/y are DECLARATIVE BINDINGS to the pill's live position (which the
        // OS drag updates directly). They also depend on our own height, so when
        // the textarea grows, height increases and y decreases (window slides
        // up) — keeping the bottom edge glued above the pill. NEVER command-assign
        // these or the binding breaks (that was the prior bug).
        y: pillWindow.positioned
           ? (pillWindow.y - root.bubbleGap - height)
           : 0
        x: pillWindow.positioned
           ? (pillWindow.x + (pillWindow.width - width) / 2)
           : 0

        ResultBubble {
            id: resultContent
            width: parent.width
            // height is NOT set here: it flows from the ColumnLayout children
            // via implicitHeight, which the window height binds to. Setting
            // height: parent.height would create a circular dependency and
            // stop the window from resizing when the textarea grows.
            text: root.result
            polishing: root.resultPolishing
            autoPolish: root.autoPolish
            polishState: root.autoPolish ? "done" : "off"
            asrModelName: root.asrModelName
            polishModelName: root.polishModelName
            onCloseRequested: {
                // 结果气泡 × = 只关闭结果气泡，pill 回到 idle（等待下次录音）
                root.result = ""
                root.resultPolishing = false
                root.bands = []
                root.rmsLevel = 0
                root.state = "idle"
            }
            onPolishRequested: {
                // 手动润色：对当前 result 调 LLM
                root.resultPolishing = true
                root.state = "polishing"
                if (voiceClient && root.result.length > 0) {
                    voiceClient.polish(root.result)
                } else {
                    root.resultPolishing = false
                    root.state = "completed"
                }
            }
        }
    }
}
