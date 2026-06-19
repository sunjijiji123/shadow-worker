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

    color: {
        if (kind === "primary") return Theme.accent
        if (kind === "danger") return Qt.rgba(239/255, 68/255, 68/255, 0.15)
        return "transparent"           // ghost
    }
    border.color: {
        if (kind === "primary") return Theme.accent
        if (kind === "danger") return Qt.rgba(239/255, 68/255, 68/255, 0.3)
        return Theme.rule              // ghost
    }
    border.width: 1
    radius: 6
    implicitWidth: btnText.implicitWidth + (small ? 20 : 28)
    implicitHeight: small ? 24 : 32

    Text {
        id: btnText
        anchors.centerIn: parent
        text: root.text
        color: {
            if (kind === "primary") return "#000000"
            if (kind === "danger") return Theme.danger
            return Theme.muted
        }
        font.pixelSize: small ? Theme.fontTiny : Theme.fontSmall
        font.weight: kind === "primary" ? Font.DemiBold : Font.Normal
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        hoverEnabled: true
        onClicked: root.clicked()
    }
}
