// Toggle.qml - matches HTML .toggle. Width 42, height 22, radius 11.
// Usage: Toggle { checked: vm.xxx; onToggled: vm.xxx = checked }

import QtQuick
import ShadowWorker

Rectangle {
    id: root

    property bool checked: false

    signal toggled(bool checked)

    width: 42
    height: 22
    radius: 11
    color: checked ? Theme.accent : Theme.bg
    border.color: checked ? Theme.accent : Theme.rule
    border.width: 1

    Behavior on color { ColorAnimation { duration: 200 } }

    Rectangle {
        id: knob
        width: 16
        height: 16
        radius: 8
        anchors.verticalCenter: parent.verticalCenter
        x: checked ? parent.width - width - 4 : 4
        color: checked ? "#000000" : Theme.muted

        Behavior on x { NumberAnimation { duration: 200; easing.type: Easing.OutQuad } }
        Behavior on color { ColorAnimation { duration: 200 } }
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: {
            root.checked = !root.checked
            root.toggled(root.checked)
        }
    }
}
