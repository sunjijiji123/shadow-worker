// VolumeBar.qml - audio volume meter (HTML volume-fill + volume-value).
// level: 0..100.

import QtQuick
import QtQuick.Layouts
import ShadowWorker

RowLayout {
    id: root

    property int level: 0   // 0..100

    Layout.fillWidth: true
    spacing: 12

    // track
    Item {
        Layout.fillWidth: true
        Layout.preferredHeight: 8

        Rectangle {
            anchors.fill: parent
            color: Theme.bg
            radius: 4
            clip: true

            Rectangle {
                width: parent.width * (root.level / 100)
                height: parent.height
                color: Theme.accent

                Behavior on width { NumberAnimation { duration: 100 } }
            }
        }
    }

    // percentage label
    Text {
        text: root.level + "%"
        color: Theme.muted
        font.pixelSize: 12
        Layout.preferredWidth: 36
        horizontalAlignment: Text.AlignRight
    }
}
