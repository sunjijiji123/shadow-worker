// Badge.qml - small status pill (HTML .badge).
// .badge: inline-block, padding 2x6, radius 4, 11px bold.
// .badge-warn: red bg/text. .badge-ok: accent bg/text.
// kind: "warn" | "ok" (default).

import QtQuick
import ShadowWorker

Rectangle {
    id: root

    property string text: ""
    property string kind: "warn"   // warn | ok

    color: kind === "warn"
           ? Qt.rgba(239/255, 68/255, 68/255, 0.15)
           : Qt.rgba(16/255, 185/255, 129/255, 0.15)

    radius: 4
    implicitWidth: badgeTxt.implicitWidth + 12
    implicitHeight: badgeTxt.implicitHeight + 4

    Text {
        id: badgeTxt
        anchors.centerIn: parent
        text: root.text
        color: root.kind === "warn" ? Theme.danger : Theme.accent
        font.pixelSize: 11
        font.weight: Font.DemiBold
    }
}
