// Button.qml - matches HTML .btn (primary/ghost/danger/sm).
// Usage: Button { text: "Save"; kind: "primary"; onClicked: ... }

import QtQuick
import ShadowWorker

Rectangle {
    id: root

    property string text: ""
    property string kind: "ghost"     // primary | ghost | danger
    property bool small: false
    // loading=true 时按钮显示 "..."、降低不透明度并禁用点击，
    // 用于异步操作（如 Save & Check、Check for Updates）的视觉反馈。
    property bool loading: false

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

    // loading 时整体变暗，文本末尾追加 "..."，让用户看到"正在处理"。
    opacity: loading ? 0.6 : 1.0
    Behavior on opacity { NumberAnimation { duration: 150 } }

    Text {
        id: btnText
        anchors.centerIn: parent
        text: root.loading ? (root.text + " ...") : root.text
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
        // loading 期间禁用点击，防止重复触发异步操作。
        enabled: !root.loading
        onClicked: root.clicked()
    }
}
