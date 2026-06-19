// QuickSwitchRow.qml - one row of the quick-switches card (title + desc + toggle).
// Matches HTML .quick-switch.

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Rectangle {
    id: root

    property string title: ""
    property string desc: ""
    property bool checked: false

    signal toggled(bool checked)

    Layout.fillWidth: true
    height: 56
    color: "transparent"

    Rectangle {
        // bottom divider
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.right: parent.right
        height: 1
        color: Theme.rule
    }

    RowLayout {
        anchors.fill: parent
        spacing: 12

        ColumnLayout {
            spacing: 2
            Text {
                text: root.title
                color: Theme.ink
                font.pixelSize: Theme.fontBody
            }
            Text {
                text: root.desc
                color: Theme.muted
                font.pixelSize: 12
            }
        }

        Item { Layout.fillWidth: true }

        Toggle {
            checked: root.checked
            onToggled: root.toggled(checked)
        }
    }
}
