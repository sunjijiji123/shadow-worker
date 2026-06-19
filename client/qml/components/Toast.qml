// Toast.qml - global lightweight toast, matches HTML .toast.
// Usage: Toast { id: toast } then toast.show("Saved")

import QtQuick
import ShadowWorker

Rectangle {
    id: root

    property string message: ""

    function show(text) {
        message = text
        showAnim.restart()
        hideTimer.restart()
    }

    visible: opacity > 0
    opacity: 0
    color: Theme.bg3
    border.color: Theme.rule
    border.width: 1
    radius: 8
    width: toastText.implicitWidth + 28
    height: 38
    z: 1000

    Text {
        id: toastText
        anchors.centerIn: parent
        text: root.message
        color: Theme.ink
        font.pixelSize: Theme.fontSmall
    }

    SequentialAnimation {
        id: showAnim
        NumberAnimation { target: root; property: "opacity"; to: 1; duration: 250 }
    }

    Timer {
        id: hideTimer
        interval: 2500
        repeat: false
        onTriggered: hideAnim.restart()
    }

    NumberAnimation {
        id: hideAnim
        target: root
        property: "opacity"
        to: 0
        duration: 250
    }
}
