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
    property bool autoPolish: true
    property bool resultPolishing: false

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
    }
    function close() {
        switch (state) {
            case "listening":
            case "transcribing":
                hide()
                break
            case "polishing":
                polishTimer.stop()
                resultPolishing = false
                state = "completed"
                break
            case "completed":
            default:
                hide()
                break
        }
    }
    function advance() {
        switch (state) {
            case "listening":
                transcript = demoTranscript
                state = "transcribing"
                break
            case "transcribing":
                state = "polishing"
                if (autoPolish) {
                    result = demoResult
                    resultPolishing = true
                    polishTimer.restart()
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

    Timer {
        id: polishTimer
        interval: 1800
        repeat: false
        onTriggered: {
            resultPolishing = false
            state = "completed"
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
            state: root.state
            transcript: root.transcript
            onCloseRequested: root.close()
        }

        // drag the pill window. resultWindow follows via its x/y bindings
        // (they read pillWindow.x/y), so we don't reposition it manually.
        MouseArea {
            anchors.fill: parent
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
        visible: root.anyVisible && root.state !== "listening"
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
            onCloseRequested: root.close()
            onPolishRequested: {
                root.resultPolishing = true
                polishTimer.restart()
            }
        }
    }
}
