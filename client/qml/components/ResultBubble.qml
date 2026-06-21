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

    // 润色标签的强调色：accent 绿（与 Copy copied 按钮一致）。
    readonly property color polishAccent: "#10B981"

    // ---- API ----
    property string text: ""             // the transcript / polished result
    property bool polishing: false       // overlay shown when true
    // 已润色标记：手动/自动润色完成后为 true。控制 Polish 按钮状态。
    property bool polished: false
    property bool autoPolish: false      // when true, icon is done + non-interactive
    // degradedHint: injectMode=auto 注入失败降级时为 true，顶部显示提示条。
    property bool degradedHint: false
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
        anchors.topMargin: 10          // 缩小气泡顶部到 Polish 行的间距
        anchors.leftMargin: 18
        anchors.rightMargin: 18
        spacing: 0

        // ---- degraded hint（injectMode=auto 注入失败降级时的提示条）----
        // 黄色背景小条，提示用户"未检测到输入框，已切回预览模式"。
        Rectangle {
            Layout.fillWidth: true
            Layout.bottomMargin: degradedHint ? 8 : 0
            visible: root.degradedHint
            height: 24
            radius: 4
            color: Qt.rgba(245/255, 158/255, 11/255, 0.15)  // amber 半透明
            border.color: Qt.rgba(245/255, 158/255, 11/255, 0.4)
            border.width: 1

            Text {
                anchors.centerIn: parent
                text: qsTr("No input field detected — switched to preview")
                color: "#F59E0B"
                font.pixelSize: 11
                font.weight: Font.DemiBold
            }
        }

        // ---- top row: Polish clickLabel (left-aligned to textarea edge) ----
        // 轻量的"魔法棒"可点击标签（镂空线框图标），替代突兀的实心按钮。
        // 未润色：灰色镂空魔法棒 + 灰色"润色"；已润色：蓝色镂空魔法棒 + 蓝色"已润色"。
        // 润色进行中（polishing）时灰掉不可点。
        RowLayout {
            Layout.fillWidth: true
            Layout.topMargin: 0
            spacing: 4

            Item {
                width: polishRow.implicitWidth
                height: polishRow.implicitHeight
                // 只在润色进行中（polishing）时变暗；autoPolish 不影响透明度
                // （自动润色模式下标签也应清晰可见，只是不可重复点击）。
                opacity: root.polishing ? 0.4 : 1.0

                Row {
                    id: polishRow
                    spacing: 4
                    layoutDirection: Qt.LeftToRight

                    Image {
                        sourceSize.width: 14
                        sourceSize.height: 14
                        width: 14; height: 14
                        anchors.verticalCenter: parent.verticalCenter
                        // 未润色用线框魔法棒，已润色用亮蓝实心魔法棒
                        source: root.polished
                                ? "qrc:/qt/qml/ShadowWorker/qml/icons/polish_active.svg"
                                : "qrc:/qt/qml/ShadowWorker/qml/icons/polish.svg"
                    }
                    Text {
                        text: root.polished ? qsTr("Polished") : qsTr("Polish")
                        color: root.polished ? root.polishAccent : Theme.muted
                        font.pixelSize: 12
                        font.weight: Font.DemiBold
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: (!root.polished && !root.polishing && !root.autoPolish)
                                 ? Qt.PointingHandCursor : Qt.ArrowCursor
                    enabled: !root.polished && !root.polishing && !root.autoPolish
                    onClicked: root.polishRequested()
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
            Layout.topMargin: 4          // 缩小 Polish 行到文本框的间距
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
                        // 未润色显示"No polish"；润色后才显示实际使用的模型名。
                        // polishModelName 是配置的润色模型（由外部传入），
                        // 仅在 polished=true 时展示，体现"这次结果用 X 模型润色过"。
                        text: root.polished && root.polishModelName !== ""
                              ? root.polishModelName : qsTr("No polish")
                        color: Theme.muted
                        font.pixelSize: 10
                    }
                }
            }

            Item { Layout.fillWidth: true }

            // right: Copy / Close（左到右顺序）。
            // Polish 按钮已移到气泡右上角（浮动），避免 Polish→Polished 文字
            // 变宽撑破 actions row 导致文本框超界。
            Row {
                spacing: 8
                Layout.alignment: Qt.AlignRight | Qt.AlignTop

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
