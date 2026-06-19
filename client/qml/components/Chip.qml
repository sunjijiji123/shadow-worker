// Chip.qml - matches HTML .chip/.filter-chip. Selectable (active state).
// Optional color dot (dotColor) for category filter chips.

import QtQuick
import ShadowWorker

Rectangle {
    id: root

    property string text: ""
    property bool checked: false
    property color dotColor: "transparent"   // if set, shows an 8x8 color square before text

    signal clicked()

    color: checked ? Theme.accentBg2 : Theme.bg
    border.color: checked ? Theme.accent : Theme.rule
    border.width: 1
    radius: 5
    implicitWidth: chipRow.implicitWidth + 20
    implicitHeight: 30

    Row {
        id: chipRow
        anchors.centerIn: parent
        spacing: 5

        Rectangle {
            visible: root.dotColor !== Qt.color("transparent")
            width: 8; height: 8; radius: 2
            color: root.dotColor
            anchors.verticalCenter: parent.verticalCenter
        }

        Text {
            text: root.text
            color: checked ? Theme.accent : Theme.muted
            font.pixelSize: Theme.fontTiny
            anchors.verticalCenter: parent.verticalCenter
        }
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }
}
