// ScreenshotResultWindow.qml - 截图 + VLM 识别后的独立结果窗口。
//
// 用途：用户用"快捷工具-桌面截图"框选一块区域后，若开启"截图后自动 VLM
// 分析"，截图框关闭后本窗口弹出，显示：
//   - 截图缩略图（用户框选的那块）
//   - VLM 识别摘要（loading 时显示加载动画）
//   - [复制摘要] / [重新截图] / [关闭] 按钮
//
// 这是一个独立的无边框置顶 Window，不依附主窗口（避免主窗口隐藏时跟着
// 消失）。定位在屏幕右下角（任务栏上方）。
//
// 驱动：main.qml 的 collectionClient.imageAnalyzed 信号回填 summary。

import QtQuick
import QtQuick.Window
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Window {
    id: root
    title: qsTr("Screenshot Result")

    // ---- 输入属性（由 main.qml 设置）----
    property string imagePath: ""   // 用户框选并保存的 PNG 路径
    property string summary: ""     // VLM 摘要（空=分析中或失败）
    property string errorText: ""   // 非空=分析失败
    property bool analyzing: true   // true=VLM 还在分析，显示 loading

    // ---- 窗口外观 ----
    flags: Qt.FramelessWindowHint | Qt.Tool | Qt.WindowStaysOnTopHint
    color: "transparent"
    width: 420
    height: 320
    minimumWidth: 420
    minimumHeight: 320

    // 初始定位：主屏右下角，离右边缘和任务栏各留 16px。
    // 用 C++ 暴露的 windowHelper.primaryAvailableGeometry()（与 RecordingWindow
    // 同范式，已验证可靠），避免 Screen attached property 的不确定性。
    // 必须在 visible 切换前设，避免出现瞬间闪在 (0,0)。
    function positionAtBottomRight() {
        if (!windowHelper) return
        var wa = windowHelper.primaryAvailableGeometry()
        x = wa.x + wa.width - width - 16
        y = wa.y + wa.height - height - 16
    }

    onVisibleChanged: {
        if (visible) {
            // 解绑 transientParent，使本窗口不再跟随主窗口：否则显示/激活
            // 本窗口时 Qt 会把主窗口一起提到前台（瞬态子窗口行为）。
            // 设为 null 必须在显示后立即做，让窗口脱离主窗口的瞬态关系。
            // （与 RecordingWindow 的 refreshVisibility 同范式）
            transientParent = null
            positionAtBottomRight()
            raise()
            requestActivate()
        }
    }

    // 窗口首次创建时就解绑 transientParent，确保从一开始就独立于主窗口，
    // 不会在弹出时把主窗口带出来。
    Component.onCompleted: transientParent = null

    // ---- 背景圆角卡片 ----
    Rectangle {
        anchors.fill: parent
        radius: 12
        color: Theme.bg3
        border.color: Theme.rule
        border.width: 1

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 16
            spacing: 12

            // ---- 顶部：标题（拖拽手柄） + 关闭按钮 ----
            // 整个标题行作为窗口拖拽区域，调 windowHelper.startDrag 走原生
            // 窗口拖拽（与 RecordingWindow 同范式，流畅无抖动）。
            RowLayout {
                Layout.fillWidth: true
                spacing: 8

                Text {
                    text: qsTr("Screenshot Analysis")
                    color: Theme.ink
                    font.pixelSize: 14
                    font.bold: true
                    Layout.fillWidth: true

                    // 拖拽手柄：按住标题文字即可拖动窗口。调 windowHelper.startDrag
                    // 走原生窗口拖拽（与 RecordingWindow 同范式，流畅无抖动）。
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.SizeAllCursor
                        onPressed: {
                            if (windowHelper) windowHelper.startDrag(root)
                        }
                    }
                }
                // 关闭按钮（×）
                Rectangle {
                    width: 24
                    height: 24
                    radius: 4
                    color: closeMa.containsMouse ? Theme.bg2 : "transparent"
                    Text {
                        anchors.centerIn: parent
                        text: "\u00D7"  // ×
                        color: Theme.muted
                        font.pixelSize: 18
                    }
                    MouseArea {
                        id: closeMa
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.hide()
                    }
                }
            }

            // ---- 缩略图 ----
            Rectangle {
                id: thumbFrame
                Layout.fillWidth: true
                Layout.preferredHeight: 112
                Layout.maximumHeight: 112
                Layout.minimumHeight: 112
                Layout.fillHeight: false
                color: Theme.bg
                radius: 6
                clip: true

                Image {
                    anchors.centerIn: parent
                    width: 380
                    height: 104
                    fillMode: Image.PreserveAspectFit
                    source: root.imagePath !== "" ? "file:///" + root.imagePath : ""
                    smooth: true
                    clip: true
                }
            }

            // ---- 摘要区 ----
            Rectangle {
                Layout.fillWidth: true
                Layout.fillHeight: true
                color: Theme.bg2
                radius: 6

                // loading 态：3 个蓝色跳动小球 + 文字（移植自 RecordingBubble
                // 的 polishing 动画，保持视觉一致性）。
                Column {
                    anchors.centerIn: parent
                    spacing: 10
                    visible: root.analyzing && root.errorText === ""

                    // 三蓝点跳动：每个 6×6 圆点，y 在 5↔13 弹跳，按 index 错开。
                    // dot 没有 anchors，y 才能被动画驱动。
                    Row {
                        anchors.horizontalCenter: parent.horizontalCenter
                        spacing: 4

                        Repeater {
                            model: 3
                            delegate: Item {
                                width: 6; height: 24
                                Rectangle {
                                    x: 0
                                    y: 9
                                    width: 6; height: 6; radius: 3
                                    color: "#3B82F6"
                                    SequentialAnimation on y {
                                        loops: Animation.Infinite
                                        PauseAnimation { duration: index * 120 }
                                        NumberAnimation {
                                            from: 5; to: 13
                                            duration: 275; easing.type: Easing.InOutQuad
                                        }
                                        NumberAnimation {
                                            from: 13; to: 5
                                            duration: 275; easing.type: Easing.InOutQuad
                                        }
                                    }
                                }
                            }
                        }
                    }

                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: qsTr("Analyzing with VLM...")
                        color: Theme.muted
                        font.pixelSize: 12
                    }
                }

                // 错误态
                Text {
                    anchors.fill: parent
                    anchors.margins: 8
                    visible: root.errorText !== ""
                    text: qsTr("Analysis failed: ") + root.errorText
                    color: Theme.danger
                    font.pixelSize: 12
                    wrapMode: Text.WordWrap
                    verticalAlignment: Text.AlignVCenter
                }

                // 结果态（可滚动）：用 Flickable + Text，避免项目自定义
                // TextArea 组件的属性差异（自定义 TextArea 没有 font/background）。
                // 只读展示，不需要编辑能力。
                Flickable {
                    anchors.fill: parent
                    anchors.margins: 8
                    visible: !root.analyzing && root.errorText === "" && root.summary !== ""
                    contentWidth: width
                    contentHeight: resultText.implicitHeight
                    clip: true
                    flickableDirection: Flickable.VerticalFlick

                    Text {
                        id: resultText
                        width: parent.width
                        text: root.summary
                        color: Theme.ink
                        font.pixelSize: 12
                        wrapMode: Text.WordWrap
                    }
                }
            }

            // ---- 底部按钮栏 ----
            RowLayout {
                Layout.fillWidth: true
                spacing: 8

                // 复制摘要
                Rectangle {
                    Layout.preferredHeight: 32
                    Layout.fillWidth: true
                    radius: 6
                    color: copyMa.containsMouse ? Theme.accentDim : Theme.accent
                    opacity: (root.summary !== "" && root.errorText === "" && !root.analyzing) ? 1.0 : 0.4

                    Text {
                        anchors.centerIn: parent
                        text: qsTr("Copy Summary")
                        color: "white"
                        font.pixelSize: 12
                        font.bold: true
                    }
                    MouseArea {
                        id: copyMa
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            if (root.summary === "" || root.errorText !== "") return
                            copySink.text = root.summary
                            copySink.selectAll()
                            copySink.copy()
                            var win = ApplicationWindow.window
                            if (win && win.toast) win.toast(qsTr("Summary copied"))
                        }
                    }
                }

                // 重新截图
                Rectangle {
                    Layout.preferredHeight: 32
                    Layout.fillWidth: true
                    radius: 6
                    color: retakeMa.containsMouse ? Theme.bg2 : Theme.rule

                    Text {
                        anchors.centerIn: parent
                        text: qsTr("Retake")
                        color: Theme.ink
                        font.pixelSize: 12
                    }
                    MouseArea {
                        id: retakeMa
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            root.hide()
                            var win = ApplicationWindow.window
                            if (win && win.startScreenshot) win.startScreenshot()
                        }
                    }
                }
            }
        }
    }

    // hidden TextInput 用作剪贴板通道（QML 标准 hack，与 SystemPage 同范式）
    TextInput {
        id: copySink
        visible: false
        readOnly: true
    }
}
