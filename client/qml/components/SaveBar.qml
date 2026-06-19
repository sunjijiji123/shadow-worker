// SaveBar.qml - settings bottom unified save bar, matches HTML .settings-save-bar.
// Only [Save], no "Reset". Floats at content bottom with gradient bg.

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Rectangle {
    id: root

    signal saveRequested()

    visible: opacity > 0
    anchors.left: parent.left
    anchors.right: parent.right
    anchors.bottom: parent.bottom
    anchors.leftMargin: Theme.sidebarWidth
    height: 64
    z: 10
    gradient: Gradient {
        orientation: Gradient.Vertical
        GradientStop { position: 0.0; color: "transparent" }
        GradientStop { position: 0.3; color: Theme.bg }
        GradientStop { position: 1.0; color: Theme.bg }
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
