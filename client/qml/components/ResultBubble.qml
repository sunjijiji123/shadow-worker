// ResultBubble.qml - the result bubble above the recording pill.
// (HTML .result-bubble)
// Structure: header (hint + polish icon) + textarea + (polishing overlay) + actions.
// .result-bubble: bg3, rule border, radius 12, padding 14x18, width 320.
// The bubble is positioned above the recording pill by its parent (main.qml).

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Rectangle {
    id: root

    // ---- API ----
    property string text: ""             // the transcript / polished result
    property bool polishing: false       // overlay shown when true
    // polish-icon state: "off" (muted, clickable) | "done" (accent, locked)
    // auto-polish on -> starts as "done" and locked
    property string polishState: "off"   // off | done
    property bool autoPolish: false      // when true, icon is done + non-interactive
    // model info shown at bottom-right
    property string asrModelName: ""     // e.g. "ggml-small" or "mimo-v2.5-asr"
    property string polishModelName: ""  // e.g. "gpt-4o" (empty = no polish)

    signal copyRequested()
    signal closeRequested()
    signal polishRequested()   // manual polish (icon clicked, only when not done/auto)

    // 把结果文字复制到系统剪贴板。用一个隐藏的 TextEdit 调用 copy()：
    // QML 没有直接的 clipboard API，这是桌面平台最可靠的纯 QML 方式。
    // 复制的是 resultEdit 当前显示的文字（与 root.text 一致，单向绑定）。
    // 复制反馈由 Copy 按钮自身状态变化呈现（不弹 toast）。
    function copyToClipboard() {
        clipHelper.text = resultEdit.text
        clipHelper.selectAll()
        clipHelper.copy()
    }

    // .result-bubble
    width: 320
    implicitHeight: content.implicitHeight + 28   // padding 14*2
    radius: 12
    color: Theme.bg3
    border.color: Theme.rule
    border.width: 1
    clip: true

    // effective polish state: autoPolish forces "done"
    property bool polishDone: autoPolish || polishState === "done"

    // 隐藏的 TextEdit，仅用于 copy() 把文字送进系统剪贴板。
    // visible:false 但不能 enabled:false（copy 需要它可操作）。
    TextEdit {
        id: clipHelper
        visible: false
        text: ""
    }

    // NOTE: NO anchors.fill — that would pin the layout to the parent's size and
    // stop implicitHeight from reflecting the children's real heights (which
    // broke textarea-resize height propagation -> text clipping + wrong height).
    // We anchor only width + top; height flows from children via implicitHeight.
    ColumnLayout {
        id: content
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: 14
        anchors.leftMargin: 18
        anchors.rightMargin: 18
        spacing: 0

        // ---- header: hint + polish icon ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 8

            Text {
                // .bubble-hint: 11px muted
                text: qsTr("Enter to inject · Esc to close")
                color: Theme.muted
                font.pixelSize: 11
                Layout.alignment: Qt.AlignVCenter
            }
            Item { Layout.fillWidth: true }

            // polish icon (sparkle). 18x18, stroked. done=accent, off=muted.
            // Clickable only when not done and not autoPolish.
            // HIDDEN while polishing so the spinning loader (below) shows alone.
            Item {
                width: 18; height: 18
                Layout.alignment: Qt.AlignVCenter
                visible: !root.polishing

                Canvas {
                    id: polishIcon
                    anchors.fill: parent
                    // redraw when state changes
                    onPolishDoneChanged: requestPaint()
                    property bool polishDone: root.polishDone
                    onPaint: {
                        var ctx = getContext("2d")
                        ctx.reset()
                        ctx.strokeStyle = root.polishDone ? Theme.accent : Theme.muted
                        ctx.fillStyle = "transparent"
                        ctx.lineWidth = 1.6
                        ctx.lineJoin = "round"
                        ctx.lineCap = "round"
                        // main 4-point star path (scaled from 24x24 viewBox)
                        ctx.beginPath()
                        ctx.moveTo(9, 2.5)
                        ctx.bezierCurveTo(9.3, 5.2, 11.1, 7.0, 13.7, 7.4)
                        ctx.bezierCurveTo(11.1, 7.8, 9.3, 9.6, 9, 12.3)
                        ctx.bezierCurveTo(8.7, 9.6, 6.9, 7.8, 4.3, 7.4)
                        ctx.bezierCurveTo(6.9, 7.0, 8.7, 5.2, 9, 2.5)
                        ctx.stroke()
                        // small star top-right
                        ctx.beginPath()
                        ctx.moveTo(13.1, 3.75)
                        ctx.lineTo(13.4, 5.1); ctx.lineTo(14.7, 5.4)
                        ctx.lineTo(13.4, 5.7); ctx.lineTo(13.1, 7.05)
                        ctx.lineTo(12.8, 5.7); ctx.lineTo(11.5, 5.4)
                        ctx.lineTo(12.8, 5.1); ctx.closePath()
                        ctx.stroke()
                        // small star bottom-left
                        ctx.beginPath()
                        ctx.moveTo(4.5, 11.25)
                        ctx.lineTo(4.7, 12.3); ctx.lineTo(5.75, 12.5)
                        ctx.lineTo(4.7, 12.7); ctx.lineTo(4.5, 13.75)
                        ctx.lineTo(4.3, 12.7); ctx.lineTo(3.25, 12.5)
                        ctx.lineTo(4.3, 12.3); ctx.closePath()
                        ctx.stroke()
                    }
                }

                // spinning variant shown while polishing
                Item {
                    id: spinIcon
                    anchors.fill: parent
                    visible: root.polishing
                    RotationAnimation on rotation {
                        running: spinIcon.visible
                        loops: Animation.Infinite
                        from: 0; to: 360; duration: 800
                    }
                    Canvas {
                        anchors.fill: parent
                        onPaint: {
                            var ctx = getContext("2d")
                            ctx.reset()
                            ctx.strokeStyle = Theme.accent
                            ctx.lineWidth = 2
                            ctx.lineCap = "round"
                            // sun-rays (8 short lines from center)
                            var cx = 9, cy = 9
                            var rays = [[9,3],[9,15],[3,9],[15,9],
                                        [4.8,4.8],[13.2,13.2],[4.8,13.2],[13.2,4.8]]
                            for (var i = 0; i < rays.length; i++) {
                                ctx.beginPath()
                                ctx.moveTo(cx, cy)
                                ctx.lineTo(rays[i][0], rays[i][1])
                                ctx.stroke()
                            }
                        }
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: (!root.polishDone && !root.polishing) ? Qt.PointingHandCursor : Qt.ArrowCursor
                    hoverEnabled: true
                    onClicked: {
                        // manual polish: only when not done and not auto-polish
                        if (root.polishDone || root.autoPolish || root.polishing) return
                        root.polishRequested()
                    }
                }
            }
        }

        // ---- textarea (the result text, display + copy) ----
        // 单向显示：text 绑定到 root.text（即 RecordingWindow.result）。
        // 不回写 onTextEdited —— 双向回写会和外部清空 result 冲突，导致第二次
        // 识别/close 后仍显示旧文字（AGENTS.md 坑 #2）。用户如需修改，复制后改。
        // resize 仍用自定义 TextArea（handle 在右上角，向上拖动放大）。
        TextArea {
            id: resultEdit
            Layout.fillWidth: true
            Layout.topMargin: 8
            text: root.text
            minHeight: 80
            frameColor: Theme.bg2
            resizeEdge: "top"
            opacity: root.polishing ? 0.35 : 1.0
            // 注意：不加 onTextEdited 回写。
        }

        // ---- actions row: model info (left) + Copy/Close (right) ----
        RowLayout {
            Layout.fillWidth: true
            Layout.topMargin: 12
            spacing: 8

            // left: model info (ASR + Polish)
            ColumnLayout {
                Layout.alignment: Qt.AlignLeft | Qt.AlignBottom
                spacing: 2

                Row {
                    spacing: 4
                    visible: root.asrModelName !== ""
                    Text {
                        text: "ASR:"
                        color: Theme.muted
                        font.pixelSize: 10
                        font.weight: Font.DemiBold
                    }
                    Text {
                        text: root.asrModelName
                        color: Theme.muted
                        font.pixelSize: 10
                    }
                }
                Row {
                    spacing: 4
                    Text {
                        text: "Polish:"
                        color: Theme.muted
                        font.pixelSize: 10
                        font.weight: Font.DemiBold
                    }
                    Text {
                        text: root.polishModelName !== "" ? root.polishModelName : qsTr("(disabled)")
                        color: Theme.muted
                        font.pixelSize: 10
                    }
                }
            }

            Item { Layout.fillWidth: true }

            // right: Copy / Close
            Row {
                spacing: 8
                layoutDirection: Qt.RightToLeft
                Layout.alignment: Qt.AlignRight | Qt.AlignTop

                Button {
                    text: qsTr("Close")
                    kind: "ghost"
                    small: true
                    onClicked: root.closeRequested()
                }
                Button {
                    // 点击后文字短暂变成 "Copied" 并高亮，1.2s 后恢复。
                    // 用按钮自身状态做反馈，不弹 toast。
                    id: copyBtn
                    text: copyBtn.copied ? qsTr("Copied!") : qsTr("Copy")
                    kind: copyBtn.copied ? "primary" : "ghost"
                    small: true
                    property bool copied: false
                    onClicked: {
                        root.copyToClipboard()
                        copyBtn.copied = true
                        copyResetTimer.restart()
                    }
                    Timer {
                        id: copyResetTimer
                        interval: 1200
                        repeat: false
                        onTriggered: copyBtn.copied = false
                    }
                }
            }
        }
    }

    // ---- polishing overlay (.polish-overlay) ----
    // absolute, covers whole bubble, semi-transparent, spinner + label
    Rectangle {
        anchors.fill: parent
        visible: root.polishing
        color: Qt.rgba(0.118, 0.118, 0.133, 0.55)   // rgba(30,30,34,0.55)
        radius: 12
        z: 2

        ColumnLayout {
            anchors.centerIn: parent
            spacing: 10

            // spinner: 22px ring, accent top border, spins
            Item {
                width: 22; height: 22
                Layout.alignment: Qt.AlignHCenter
                RotationAnimation on rotation {
                    running: root.polishing
                    loops: Animation.Infinite
                    from: 0; to: 360; duration: 800
                }
                Canvas {
                    anchors.fill: parent
                    onPaint: {
                        var ctx = getContext("2d")
                        ctx.reset()
                        ctx.strokeStyle = Theme.rule
                        ctx.lineWidth = 2
                        ctx.beginPath()
                        ctx.arc(11, 11, 9, 0, 2 * Math.PI)
                        ctx.stroke()
                        ctx.strokeStyle = Theme.accent
                        ctx.beginPath()
                        ctx.arc(11, 11, 9, 0, 0.5 * Math.PI)
                        ctx.stroke()
                    }
                }
            }

            Text {
                text: qsTr("AI polishing...")
                color: Theme.ink
                font.pixelSize: 13
                Layout.alignment: Qt.AlignHCenter
            }
        }
    }
}
