// RetryProgressDialog.qml - modal loading dialog for VLM retry.
// Shows a spinner + "正在识别..." text while the backend re-identifies a screenshot.
// Has a 2.5-minute timeout to prevent UI deadlock (backend single-task timeout is 2min).
// User can also click "取消等待" to close (backend request continues, result still toasts).
//
// Usage:
//   RetryProgressDialog { id: retryProgressDialog; parent: Overlay.overlay }
//   retryProgressDialog.open()  // before calling viewModel.retryVLMFailures()
//   // close() is called by retryFinished signal handler in main.qml

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Dialog {
    id: root

    modal: true
    // NoAutoClose: 不能 ESC 关闭、不能点外部关闭（只能超时或手动取消）。
    closePolicy: Dialog.NoAutoClose
    anchors.centerIn: parent
    width: 360
    padding: 24
    topPadding: 32
    bottomPadding: 24

    background: Rectangle {
        color: Theme.bg3
        border.color: Theme.rule
        border.width: 1
        radius: 12
    }

    // 2.5 分钟超时 Timer（后端单任务 2 分钟 + 0.5 分钟余量）。
    // open() 时启动，close() 时停止。超时后自动关闭 + 回调 onTimeout。
    property int timeoutMs: 150000
    signal timeout()

    onOpened: timeoutTimer.start()
    onClosed: timeoutTimer.stop()

    Timer {
        id: timeoutTimer
        interval: root.timeoutMs
        repeat: false
        onTriggered: {
            root.close()
            root.timeout()
        }
    }

    contentItem: ColumnLayout {
        spacing: 14

        // spinner: 28px ring, accent 1/4 arc, 800ms/round.
        // 范式照抄 ResultBubble.qml:263-286 的 polishing spinner。
        Item {
            width: 28; height: 28
            Layout.alignment: Qt.AlignHCenter

            RotationAnimation on rotation {
                running: root.visible
                loops: Animation.Infinite
                from: 0; to: 360; duration: 800
            }
            Canvas {
                anchors.fill: parent
                onPaint: {
                    var ctx = getContext("2d")
                    ctx.reset()
                    // 灰色底环
                    ctx.strokeStyle = Theme.rule
                    ctx.lineWidth = 3
                    ctx.beginPath()
                    ctx.arc(14, 14, 11, 0, 2 * Math.PI)
                    ctx.stroke()
                    // accent 色 1/4 弧（旋转部分）
                    ctx.strokeStyle = Theme.accent
                    ctx.beginPath()
                    ctx.arc(14, 14, 11, 0, 0.5 * Math.PI)
                    ctx.stroke()
                }
            }
        }

        Text {
            text: qsTr("正在识别...")
            color: Theme.ink
            font.pixelSize: 14
            font.weight: Font.DemiBold
            Layout.alignment: Qt.AlignHCenter
        }

        Text {
            text: qsTr("正在重新识别该截图，请稍候")
            color: Theme.muted
            font.pixelSize: 12
            Layout.alignment: Qt.AlignHCenter
            Layout.fillWidth: true
            wrapMode: Text.WordWrap
            horizontalAlignment: Text.AlignHCenter
        }

        // 取消等待按钮（不中断后端请求，只关闭弹窗）。
        Button {
            text: qsTr("取消等待")
            kind: "ghost"
            Layout.alignment: Qt.AlignHCenter
            onClicked: root.close()
        }
    }
}
