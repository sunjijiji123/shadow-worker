// Radio.qml - single-select row, matches HTML .radio.
// Usage: Radio { text: "..."; checked: ...; onClicked: ... }

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Rectangle {
    id: root

    property string text: ""
    property bool checked: false

    signal clicked()

    Layout.fillWidth: true
    color: checked ? Theme.accentBg : Theme.bg
    border.color: checked ? Theme.accent : Theme.rule
    border.width: 1
    radius: 8
    implicitHeight: 40

    RowLayout {
        anchors.fill: parent
        anchors.leftMargin: 12
        anchors.rightMargin: 12
        spacing: 10

        Rectangle {
            width: 16
            height: 16
            radius: 8
            color: "transparent"
            border.color: checked ? Theme.accent : Theme.rule
            border.width: 2

            Rectangle {
                visible: root.checked
                width: 6
                height: 6
                radius: 3
                anchors.centerIn: parent
                color: Theme.accent
            }
        }

        Text {
            text: root.text
            color: checked ? Theme.ink : Theme.muted
            font.pixelSize: Theme.fontBody
            Layout.fillWidth: true
        }
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }
}
