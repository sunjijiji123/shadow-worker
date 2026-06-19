// MicTestButton.qml - mic test button matching HTML .mic-test-btn.
// Green pill with mic icon; testing state turns red with pulse dot.

import QtQuick
import ShadowWorker

Rectangle {
    id: root

    property bool testing: false
    signal clicked()

    height: 36
    width: row.implicitWidth + 28
    radius: 6
    color: testing ? Qt.rgba(239/255, 68/255, 68/255, 0.12)
                   : (ma.containsMouse ? Qt.rgba(16/255, 185/255, 129/255, 0.2)
                                       : Qt.rgba(16/255, 185/255, 129/255, 0.1))
    border.color: testing ? Theme.danger : Theme.accent
    border.width: 1

    Behavior on color { ColorAnimation { duration: 200 } }
    Behavior on border.color { ColorAnimation { duration: 200 } }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 6

        // pulse dot (visible only when testing)
        Rectangle {
            visible: root.testing
            width: 7; height: 7; radius: 3.5
            color: root.testing ? Theme.danger : Theme.accent
            anchors.verticalCenter: parent.verticalCenter

            SequentialAnimation on opacity {
                running: root.testing
                loops: Animation.Infinite
                NumberAnimation { from: 1; to: 0.45; duration: 600 }
                NumberAnimation { from: 0.45; to: 1; duration: 600 }
            }
            SequentialAnimation on scale {
                running: root.testing
                loops: Animation.Infinite
                NumberAnimation { from: 1; to: 0.85; duration: 600 }
                NumberAnimation { from: 0.85; to: 1; duration: 600 }
            }
        }

        // mic icon (SVG path)
        Canvas {
            width: 14; height: 14
            anchors.verticalCenter: parent.verticalCenter
            onPaint: {
                var ctx = getContext("2d")
                ctx.reset()
                ctx.strokeStyle = root.testing ? Theme.danger : Theme.accent
                ctx.lineWidth = 1.5
                ctx.fillStyle = root.testing ? Theme.danger : Theme.accent
                // simplified mic body
                ctx.beginPath()
                ctx.roundedRect(4, 1, 6, 9, 3, 3)
                ctx.fill()
                // arc
                ctx.beginPath()
                ctx.arc(7, 8, 4, 0.2, Math.PI - 0.2)
                ctx.stroke()
                // stem
                ctx.beginPath()
                ctx.moveTo(7, 12)
                ctx.lineTo(7, 14)
                ctx.stroke()
            }
        }

        Text {
            text: root.testing ? qsTr("Testing...") : qsTr("Test Microphone")
            color: root.testing ? Theme.danger
                  : (ma.containsMouse ? "#ffffff" : Theme.accent)
            font.pixelSize: 13
            anchors.verticalCenter: parent.verticalCenter
        }
    }

    MouseArea {
        id: ma
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        hoverEnabled: true
        onClicked: {
            root.testing = !root.testing
            root.clicked()
        }
    }
}
