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
    // 已润色标记：手动/自动润色完成后为 true。控制 Polish 按钮状态。
    property bool polished: false
    property bool autoPolish: false      // when true, icon is done + non-interactive
    // model info shown at bottom-left
    property string asrModelName: ""     // e.g. "ggml-small" or "mimo-v2.5-asr"
    property string polishModelName: ""  // e.g. "gpt-4o" (empty = no polish)

    signal copyRequested()
    signal closeRequested()
    signal polishRequested()   // manual polish（未润色时点击触发）

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
    // width 由外层（RecordingWindow 的 ResultBubble 实例）设为 parent.width，
    // 与录音气泡 pillWindow 保持一致宽度。这里不写死默认值，避免与外部
    // width: parent.width 绑定冲突导致宽度不一致。
    implicitHeight: content.implicitHeight + 28   // padding 14*2
    radius: 12
    color: Theme.bg3
    border.color: Theme.rule
    border.width: 1
    clip: true

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
                        // 有润色模型显示模型名，否则显示"没有润色"
                        text: root.polishModelName !== "" ? root.polishModelName : qsTr("No polish")
                        color: Theme.muted
                        font.pixelSize: 10
                    }
                }
            }

            Item { Layout.fillWidth: true }

            // right: Polish / Copy / Close（左到右顺序）
            Row {
                spacing: 8
                Layout.alignment: Qt.AlignRight | Qt.AlignTop

                // Polish 按钮：未润色时可点击触发润色；已润色时灰掉显示"已润色"。
                // polishing 期间也禁用（润色进行中）。
                Button {
                    text: root.polished ? qsTr("Polished") : qsTr("Polish")
                    kind: root.polished ? "ghost" : "primary"
                    small: true
                    enabled: !root.polished && !root.polishing && !root.autoPolish
                    onClicked: root.polishRequested()
                }
                Button {
                    // Copy：点击后文字短暂变成 "Copied!" 并高亮，1.2s 后恢复。
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
                Button {
                    text: qsTr("Close")
                    kind: "ghost"
                    small: true
                    onClicked: root.closeRequested()
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
