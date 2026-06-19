// RankBar.qml - horizontal ranking bar (used by category rank AND app rank).
// label | [###### bar ######] | value
// barWidth 0..1, barColor = category/app color.

import QtQuick
import QtQuick.Layouts
import ShadowWorker

RowLayout {
    id: root

    property string label: ""
    property string value: ""
    property real barRatio: 0.0      // 0..1
    property color barColor: Theme.accent
    property color dotColor: "transparent"   // optional leading dot (category color)

    spacing: 12
    Layout.fillWidth: true

    // optional dot
    Rectangle {
        visible: root.dotColor !== "transparent"
        width: 10
        height: 10
        radius: 2
        color: root.dotColor
        Layout.alignment: Qt.AlignVCenter
    }

    Text {
        text: root.label
        color: Theme.ink
        font.pixelSize: Theme.fontBody
        Layout.preferredWidth: 80
        elide: Text.ElideRight
    }

    Item {
        Layout.fillWidth: true
        Layout.preferredHeight: 10
        Layout.alignment: Qt.AlignVCenter

        Rectangle {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: parent.width * Math.max(root.barRatio, 0.01)   // min visible sliver
            height: 8
            radius: 4
            color: root.barColor

            Behavior on width { NumberAnimation { duration: 250; easing.type: Easing.OutQuad } }
        }
    }

    Text {
        text: root.value
        color: Theme.muted
        font.pixelSize: Theme.fontSmall
        Layout.preferredWidth: 110
        horizontalAlignment: Text.AlignRight
    }
}
