import QtQuick
import QtQuick.Window

// PinWindow: 截图置顶窗口。点截图工具条的图钉按钮后弹出。
// 支持：拖拽标题栏移动、8 方向拖拽边缘缩放、滚轮缩放、双击切换 fit/100%。
Window {
    id: root
    flags: Qt.FramelessWindowHint | Qt.Tool | Qt.WindowStaysOnTopHint
    color: "transparent"
    width: 400
    height: 300
    minimumWidth: 150
    minimumHeight: 120

    property string imagePath: ""
    property real imageScale: 1.0
    property bool fitMode: true
    property string fileName: {
        var p = imagePath.replace("file:///", "").replace("file://", "")
        return p.split("/").pop().split("\\").pop()
    }

    Component.onCompleted: {
        root.transientParent = null
        var avail = Qt.application.screens[0].availableGeometry
        root.x = avail.x + (avail.width - root.width) / 2
        root.y = avail.y + (avail.height - root.height) / 2
    }

    onVisibleChanged: {
        root.transientParent = null
        if (visible) { root.raise(); root.requestActivate() }
    }

    // ---- 外框 ----
    Rectangle {
        id: frame
        anchors.fill: parent
        color: "#2c2c31"
        border.color: "#3f3f46"
        border.width: 1
        radius: 10
        clip: true

        Column {
            anchors.fill: parent
            spacing: 0

            // ---- 标题栏 ----
            Rectangle {
                id: titleBar
                width: parent.width
                height: 36
                color: "#232327"
                radius: 10

                Rectangle {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    height: 18
                    color: "#232327"
                }

                Row {
                    anchors.fill: parent
                    anchors.leftMargin: 10
                    anchors.rightMargin: 6
                    spacing: 8

                    Text {
                        text: "\uD83D\uDCCC"
                        font.pixelSize: 14
                        anchors.verticalCenter: parent.verticalCenter
                    }

                    Text {
                        width: parent.width - 20 - 50 - 28 - 8 * 3
                        anchors.verticalCenter: parent.verticalCenter
                        text: root.fileName
                        color: "#9ca3af"
                        font.pixelSize: 12
                        elide: Text.ElideRight
                        verticalAlignment: Text.AlignVCenter
                    }

                    Rectangle {
                        width: 44; height: 20
                        anchors.verticalCenter: parent.verticalCenter
                        color: "#18181b"
                        radius: 4
                        Text {
                            anchors.centerIn: parent
                            text: Math.round(root.imageScale * 100) + "%"
                            color: root.imageScale !== 1.0 ? "#3b82f6" : "#9ca3af"
                            font.pixelSize: 11
                            font.bold: root.imageScale !== 1.0
                        }
                    }

                    Item { width: 1; height: 1; Layout.fillWidth: true }

                    Rectangle {
                        width: 24; height: 24
                        anchors.verticalCenter: parent.verticalCenter
                        radius: 5
                        color: closeArea.containsMouse ? "#ef4444" : "transparent"
                        Text {
                            anchors.centerIn: parent
                            text: "\u00d7"
                            color: closeArea.containsMouse ? "#ffffff" : "#9ca3af"
                            font.pixelSize: 16
                            font.bold: true
                        }
                        MouseArea {
                            id: closeArea
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: root.close()
                        }
                    }
                }

                // 拖拽移动窗口
                MouseArea {
                    anchors.fill: parent
                    anchors.rightMargin: 30
                    cursorShape: Qt.SizeAllCursor
                    property point startPos
                    property point startWin
                    onPressed: function(mouse) {
                        startPos = Qt.point(mouse.x, mouse.y)
                        startWin = Qt.point(root.x, root.y)
                    }
                    onPositionChanged: function(mouse) {
                        if (pressed) {
                            root.x = startWin.x + (mouse.x - startPos.x)
                            root.y = startWin.y + (mouse.y - startPos.y)
                        }
                    }
                    onDoubleClicked: {
                        root.fitMode = !root.fitMode
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: "#3f3f46" }

            // ---- 图片画布 ----
            Item {
                id: canvas
                width: parent.width
                height: parent.height - 37
                clip: true

                Rectangle {
                    anchors.fill: parent
                    color: "#111111"
                    z: -1
                }

                Image {
                    id: img
                    source: root.imagePath
                    anchors.centerIn: parent
                    fillMode: root.fitMode ? Image.PreserveAspectFit : Image.Pad
                    sourceSize.width: root.fitMode ? canvas.width : 0
                    sourceSize.height: root.fitMode ? canvas.height : 0
                    scale: root.fitMode ? 1.0 : root.imageScale
                    transformOrigin: Item.Center
                    smooth: true
                }

                WheelHandler {
                    target: canvas
                    onWheel: function(event) {
                        var delta = event.angleDelta.y > 0 ? 1.1 : (1 / 1.1)
                        var ns = root.imageScale * delta
                        if (ns < 0.1) ns = 0.1
                        if (ns > 5.0) ns = 5.0
                        root.imageScale = ns
                        root.fitMode = false
                    }
                }
            }
        }
    }

    // ---- 8 方向 resize 手柄 ----
    Repeater {
        model: [
            { edge: "left",   cx: 0,   cy: 0.5, w: 6,  h: 0,   cursor: Qt.SizeHorCursor },
            { edge: "right",  cx: 1,   cy: 0.5, w: 6,  h: 0,   cursor: Qt.SizeHorCursor },
            { edge: "top",    cx: 0.5, cy: 0,   w: 0,  h: 6,   cursor: Qt.SizeVerCursor },
            { edge: "bottom", cx: 0.5, cy: 1,   w: 0,  h: 6,   cursor: Qt.SizeVerCursor },
            { edge: "tl",     cx: 0,   cy: 0,   w: 12, h: 12,  cursor: Qt.SizeFDiagCursor },
            { edge: "tr",     cx: 1,   cy: 0,   w: 12, h: 12,  cursor: Qt.SizeBDiagCursor },
            { edge: "bl",     cx: 0,   cy: 1,   w: 12, h: 12,  cursor: Qt.SizeBDiagCursor },
            { edge: "br",     cx: 1,   cy: 1,   w: 12, h: 12,  cursor: Qt.SizeFDiagCursor }
        ]
        Item {
            property var info: modelData
            x: info.cx === 0 ? 0 : (info.cx === 1 ? root.width - info.w : root.width / 2 - info.w / 2)
            y: info.cy === 0 ? 0 : (info.cy === 1 ? root.height - info.h : root.height / 2 - info.h / 2)
            width: info.w === 0 ? root.width : info.w
            height: info.h === 0 ? root.height : info.h

            MouseArea {
                anchors.fill: parent
                cursorShape: info.cursor
                property point startMouse
                property point startWin
                property real startW
                property real startH
                onPressed: function(mouse) {
                    startMouse = root.mapToGlobal(Qt.point(mouse.x, mouse.y))
                    startWin = Qt.point(root.x, root.y)
                    startW = root.width
                    startH = root.height
                }
                onPositionChanged: function(mouse) {
                    if (!pressed) return
                    var gp = root.mapToGlobal(Qt.point(mouse.x, mouse.y))
                    var dx = gp.x - startMouse.x
                    var dy = gp.y - startMouse.y
                    var e = info.edge
                    var newW = startW, newH = startH, newX = startWin.x, newY = startWin.y

                    if (e === "left" || e === "tl" || e === "bl") {
                        newW = startW - dx; newX = startWin.x + dx
                    }
                    if (e === "right" || e === "tr" || e === "br") {
                        newW = startW + dx
                    }
                    if (e === "top" || e === "tl" || e === "tr") {
                        newH = startH - dy; newY = startWin.y + dy
                    }
                    if (e === "bottom" || e === "bl" || e === "br") {
                        newH = startH + dy
                    }

                    if (newW < root.minimumWidth) {
                        if (newX !== startWin.x) newX = startWin.x + (startW - root.minimumWidth)
                        newW = root.minimumWidth
                    }
                    if (newH < root.minimumHeight) {
                        if (newY !== startWin.y) newY = startWin.y + (startH - root.minimumHeight)
                        newH = root.minimumHeight
                    }

                    root.x = newX
                    root.y = newY
                    root.width = newW
                    root.height = newH
                }
            }
        }
    }

    Shortcut {
        sequences: ["Esc"]
        onActivated: root.close()
    }
}
