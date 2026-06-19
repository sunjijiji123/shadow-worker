// StatCard.qml - overview stat card, matches HTML .stat-card.
// Usage: StatCard { label: "..."; value: "..."; sub: "..." }

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Rectangle {
    id: root

    property string label: ""
    property string value: ""
    property string sub: ""
    property color subColor: Theme.muted

    Layout.fillWidth: true
    color: Theme.bg3
    border.color: Theme.rule
    border.width: 1
    radius: Theme.radiusCard
    implicitHeight: 116

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 4

        Text {
            text: root.label
            color: Theme.muted
            font.pixelSize: Theme.fontTiny
        }

        Item { Layout.fillHeight: true }

        Text {
            text: root.value
            color: Theme.ink
            font.pixelSize: Theme.fontStat
            font.weight: Font.Bold
        }

        Text {
            visible: root.sub !== ""
            text: root.sub
            color: root.subColor
            font.pixelSize: Theme.fontTiny
        }
    }
}
