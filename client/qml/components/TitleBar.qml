// TitleBar.qml - custom frameless title bar for the main window.
//
// Layout:   Shadow Worker                              [≡] [—] [×]
//
// - Center: app brand name (NOT translated — brand identity + keeps
//           FindWindowW("Shadow Worker") in singleinstance.cpp working).
// - Right:  menu (≡) / minimize (—) / close (×) buttons.
// - The whole bar is draggable: pressing empty space calls
//   mainWindow.startSystemMove() for native frameless drag.
//
// Language names are ALWAYS shown in their own script:
//   "简体中文" never becomes "Simplified Chinese"
//   "English"  never becomes "英语"

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

    // ==================== Right: menu / min / close ====================
    Row {
        id: rightCluster
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        spacing: 0

        // ---- menu button (≡) ----
        Rectangle {
            id: menuBtn
            height: titleBar.barHeight
            width: 46
            color: menuBtnMa.containsMouse ? "#3D3D3D" : "transparent"
            Behavior on color { ColorAnimation { duration: 80 } }

            Canvas {
                anchors.centerIn: parent
                width: 16; height: 16
                onPaint: {
                    var ctx = getContext("2d")
                    ctx.reset()
                    ctx.strokeStyle = menuBtnMa.containsMouse ? Theme.ink : Theme.muted
                    ctx.lineWidth = 1.5
                    ctx.lineCap = "round"
                    for (var i = 0; i < 3; i++) {
                        var y = 4 + i * 5
                        ctx.beginPath()
                        ctx.moveTo(2, y)
                        ctx.lineTo(14, y)
                        ctx.stroke()
                    }
                }
            }
            MouseArea {
                id: menuBtnMa
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                hoverEnabled: true
                onClicked: {
                    // menuBtn.x is relative to its Row parent; map to titleBar
                    // so popup() positions correctly (Menu's parent is titleBar).
                    var p = menuBtn.mapToItem(titleBar, 0, menuBtn.height)
                    rootMenu.popup(p.x, p.y)
                }
            }
        }

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

    // ==================== Language Menu (VS Code style) ====================
    // Uses palette for dark theming. This is the approach that worked
    // in the first iteration — keep Menu inside TitleBar, use popup()
    // with coords relative to titleBar.
    Menu {
        id: rootMenu
        palette.window: "#252526"
        palette.windowText: "#CCCCCC"
        palette.mid: "#252526"
        palette.text: "#CCCCCC"
        palette.highlightedText: "#FFFFFF"
        palette.highlight: "#094771"

        Menu {
            title: qsTr("Language")

            MenuItem {
                text: "简体中文"
                checkable: true
                checked: translator ? translator.currentLanguage === "zh_CN" : false
                onTriggered: if (translator) translator.setLanguage("zh_CN")
            }
            MenuItem {
                text: "English"
                checkable: true
                checked: translator ? translator.currentLanguage === "en" : false
                onTriggered: if (translator) translator.setLanguage("en")
            }
        }
    }
}
