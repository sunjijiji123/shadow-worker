// Button.qml - matches HTML .btn (primary/ghost/danger/sm).
// Usage: Button { text: "Save"; kind: "primary"; onClicked: ... }

import QtQuick
import ShadowWorker

Rectangle {
    id: root

    property string text: ""
    property string kind: "ghost"     // primary | ghost | danger
    property bool small: false

    signal clicked()

    // hover state (ghost buttons highlight border + text on hover)
    readonly property bool hovered: ma.containsMouse
    readonly property color ghostBorder: hovered ? Theme.accent : Theme.rule
    readonly property color ghostText: hovered ? Theme.ink : Theme.muted

    color: {
        if (kind === "primary") return Theme.accent
        if (kind === "danger") return Qt.rgba(239/255, 68/255, 68/255, 0.15)
        return "transparent"           // ghost
    }
    border.color: {
        if (kind === "primary") return Theme.accent
        if (kind === "danger") return Qt.rgba(239/255, 68/255, 68/255, 0.3)
        return ghostBorder             // ghost (hover-aware)
    }
    border.width: 1
    radius: 6
    implicitWidth: btnText.implicitWidth + (small ? 20 : 28)
    // height matches Chip (30) for layout alignment
    implicitHeight: small ? 24 : 30

    Behavior on border.color { ColorAnimation { duration: 150 } }

    Text {
        id: btnText
        anchors.centerIn: parent
        text: root.text
        color: {
            if (kind === "primary") return "#000000"
            if (kind === "danger") return Theme.danger
            return ghostText            // ghost (hover-aware)
        }
        font.pixelSize: small ? Theme.fontTiny : Theme.fontSmall
        font.weight: kind === "primary" ? Font.DemiBold : Font.Normal

        Behavior on color { ColorAnimation { duration: 150 } }
    }

    MouseArea {
        id: ma
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        hoverEnabled: true
        onClicked: root.clicked()
    }
}
