// Chip.qml - matches HTML .chip. Selectable (active state).
// Usage: Chip { text: "..."; checked: ...; onClicked: ... }

import QtQuick
import ShadowWorker

Rectangle {
    id: root

    property string text: ""
    property bool checked: false

    signal clicked()

    color: checked ? Theme.accentBg2 : Theme.bg
    border.color: checked ? Theme.accent : Theme.rule
    border.width: 1
    radius: 5
    implicitWidth: chipText.implicitWidth + 20
    implicitHeight: 24

    Text {
        id: chipText
        anchors.centerIn: parent
        text: root.text
        color: checked ? Theme.accent : Theme.muted
        font.pixelSize: Theme.fontTiny
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: root.clicked()
    }
}
