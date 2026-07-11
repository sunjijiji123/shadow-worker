// Toast.qml - global toast with status icon (success/error/warning).
// HTML .toast + SVG check icon. Extended with 3 types.
// 支持长文本自动换行：最大宽度 420px，高度随内容自适应。

import QtQuick
import ShadowWorker

Rectangle {
    id: root

    property string message: ""
    property string toastType: "success"   // success | error | warning

    // 堆叠模式：动态创建后自动显示，淡出后自动销毁。
    // 调用方只需 createObject(parent, {message: ..., toastType: ...})，
    // Toast 自己管生命周期（show → 定时 → 淡出 → destroy）。
    Component.onCompleted: {
        if (message.length > 0) show(message, toastType)
    }

    function show(text, type) {
        message = text
        if (type !== undefined) toastType = type
        else toastType = "success"
        icon.source = "qrc:/qt/qml/ShadowWorker/qml/icons/toast_" + toastType + ".svg"
        // 长文本延长显示时间（每 50 字 +1s，上限 8s）
        var len = text ? text.length : 0
        hideTimer.interval = Math.min(8000, 2500 + Math.floor(len / 50) * 1000)
        showAnim.restart()
        hideTimer.restart()
    }

    visible: opacity > 0
    opacity: 0
    color: Theme.bg3
    border.color: Theme.rule
    border.width: 1
    radius: 8
    width: Math.min(420, row.implicitWidth + 28)
    height: Math.max(40, msgText.implicitHeight + 24)
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
            id: msgText
            text: root.message
            color: Theme.ink
            font.pixelSize: 13
            anchors.verticalCenter: parent.verticalCenter
            wrapMode: Text.Wrap
            maximumLineCount: 6
            elide: Text.ElideRight
            // 留出 icon(16) + spacing(8) + padding(28) 的空间
            width: Math.min(420 - 52, msgText.implicitWidth)
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

    SequentialAnimation {
        id: hideAnim
        NumberAnimation { target: root; property: "opacity"; to: 0; duration: 250 }
        // 淡出完成后销毁自身——堆叠容器（Column）自动让后面的消息上移。
        ScriptAction { script: root.destroy() }
    }
}
