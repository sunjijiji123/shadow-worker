// SaveBar.qml - settings bottom save bar, matches HTML .settings-save-bar.
// Spans full content area width; gradient bg matches window bg2.

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Rectangle {
    id: root

    signal saveRequested()

    // parent is the content Item (already right of sidebar), so margins are 0 left.
    anchors.left: parent.left
    anchors.right: parent.right
    anchors.bottom: parent.bottom
    height: 64
    z: 10
    gradient: Gradient {
        orientation: Gradient.Vertical
        GradientStop { position: 0.0; color: Qt.rgba(0.094, 0.094, 0.102, 0) }   // transparent bg2
        GradientStop { position: 0.3; color: Theme.bg2 }
        GradientStop { position: 1.0; color: Theme.bg2 }
    }

    RowLayout {
        anchors.fill: parent
        anchors.rightMargin: Theme.contentPadding
        anchors.leftMargin: Theme.contentPadding
        anchors.topMargin: 12

        Item { Layout.fillWidth: true }

        Button {
            kind: "primary"
            text: qsTr("Save")
            onClicked: root.saveRequested()
        }
    }
}
