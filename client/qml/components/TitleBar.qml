// TitleBar.qml - custom frameless title bar for the main window.
//
// Layout:   Shadow Worker                              [—] [×]
//
// - Center: app brand name (NOT translated — brand identity + keeps
//           FindWindowW("Shadow Worker") in singleinstance.cpp working).
// - Right:  minimize (—) / close (×) buttons.
// - The whole bar is draggable: pressing empty space calls
//   mainWindow.startSystemMove() for native frameless drag.
//
// NOTE: 语言切换原在此处的 ≡ 菜单里，现已迁至 系统设置页 → 界面语言 卡片
//   （SystemPage.qml）。该菜单连同 ≡ 按钮一并移除，标题栏回归极简。

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Rectangle {
    id: titleBar

    property var window: null
    property int barHeight: 36

    height: barHeight
    color: Theme.bg2

    // bottom divider
    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 1
        color: Theme.rule
    }

    // ---- drag area (left side only, leaves room for buttons) ----
    MouseArea {
        id: dragArea
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
        anchors.rightMargin: 200
        z: 0
        onPressed: function(mouse) {
            if (window) window.startSystemMove()
            mouse.accepted = true
        }
    }

    // ==================== Center: brand name ====================
    Text {
        anchors.centerIn: parent
        text: "Shadow Worker"
        color: Theme.muted
        font.pixelSize: Theme.fontSmall
        font.weight: Font.Medium
    }

    // ==================== Right: min / close ====================
    Row {
        id: rightCluster
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        spacing: 0

        // ---- minimize button (—) ----
        Rectangle {
            id: minBtn
            height: titleBar.barHeight
            width: 46
            color: minBtnMa.containsMouse ? Theme.bg3 : "transparent"
            Behavior on color { ColorAnimation { duration: 80 } }

            Canvas {
                anchors.centerIn: parent
                width: 16; height: 16
                onPaint: {
                    var ctx = getContext("2d")
                    ctx.reset()
                    ctx.strokeStyle = minBtnMa.containsMouse ? Theme.ink : Theme.muted
                    ctx.lineWidth = 1.4
                    ctx.lineCap = "round"
                    ctx.beginPath()
                    ctx.moveTo(2, 8)
                    ctx.lineTo(14, 8)
                    ctx.stroke()
                }
            }
            MouseArea {
                id: minBtnMa
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                hoverEnabled: true
                onClicked: if (window) window.showMinimized()
            }
        }

        // ---- close button (×) ----
        Rectangle {
            id: closeBtn
            height: titleBar.barHeight
            width: 46
            color: closeBtnMa.containsMouse ? Theme.danger : "transparent"
            Behavior on color { ColorAnimation { duration: 80 } }

            Canvas {
                anchors.centerIn: parent
                width: 16; height: 16
                onPaint: {
                    var ctx = getContext("2d")
                    ctx.reset()
                    ctx.strokeStyle = closeBtnMa.containsMouse ? Theme.ink : Theme.muted
                    ctx.lineWidth = 1.4
                    ctx.lineCap = "round"
                    ctx.beginPath()
                    ctx.moveTo(3, 3)
                    ctx.lineTo(13, 13)
                    ctx.stroke()
                    ctx.beginPath()
                    ctx.moveTo(13, 3)
                    ctx.lineTo(3, 13)
                    ctx.stroke()
                }
            }
            MouseArea {
                id: closeBtnMa
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                hoverEnabled: true
                onClicked: if (window) window.hide()
            }
        }
    }
}
