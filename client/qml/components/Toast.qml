// Toast.qml - global toast with status icon (success/error/warning).
// HTML .toast + SVG check icon. Extended with 3 types.

import QtQuick
import ShadowWorker

Rectangle {
    id: root

    property string message: ""
    property string toastType: "success"   // success | error | warning

    function show(text, type) {
        message = text
        if (type !== undefined) toastType = type
        else toastType = "success"
        icon.source = "qrc:/qt/qml/ShadowWorker/qml/icons/toast_" + toastType + ".svg"
        showAnim.restart()
        hideTimer.restart()
    }

    visible: opacity > 0
    opacity: 0
    color: Theme.bg3
    border.color: Theme.rule
    border.width: 1
    radius: 8
    width: row.implicitWidth + 28
    height: 40
    z: 1000

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 8

        Image {
            id: icon
            width: 16; height: 16
            source: "qrc:/qt/qml/ShadowWorker/qml/icons/toast_success.svg"
            anchors.verticalCenter: parent.verticalCenter
        }

        Text {
            text: root.message
            color: Theme.ink
            font.pixelSize: 13
            anchors.verticalCenter: parent.verticalCenter
        }
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
