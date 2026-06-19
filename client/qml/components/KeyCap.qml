// KeyCap.qml - a keyboard-key visual (HTML .key-cap).
// Used in the personal-prompt shortcut display.
// .key-cap: min-width 28, height 28, bg2 fill, rule border, radius 6,
// 12px monospace ink text, and a 3D "box-shadow: 0 2px 0 rule" bottom edge.
// Two modes:
//   - static label (prefix key like "Ctrl")
//   - editable single char (.key-input: width 28, centered, 13px mono)

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Rectangle {
    id: root

    // ---- API ----
    property string text: ""
    property bool editable: false
    property string keyChar: ""    // for editable mode

    signal keyCharEdited(string newKey)

    // .key-cap: min-width 28, height 28
    width: Math.max(28, capContent.implicitWidth + 12)
    height: 28
    radius: 6
    color: Theme.bg2
    border.color: Theme.rule
    border.width: 1

    // 3D bottom edge: "box-shadow: 0 2px 0 rule" -> a 2px-tall rule-colored
    // rectangle peeking 2px below the cap, shifted down by 2px.
    Rectangle {
        z: -1
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.bottom
        anchors.topMargin: -1     // overlap 1px so no gap; 2px shows below
        height: 3
        color: Theme.rule
        radius: 6
    }

    // content: static label OR editable single-char input
    Item {
        id: capContent
        anchors.centerIn: parent
        width: editable ? keyInput.implicitWidth : lbl.implicitWidth
        height: editable ? keyInput.implicitHeight : lbl.implicitHeight

        Text {
            id: lbl
            visible: !root.editable
            anchors.centerIn: parent
            text: root.text
            color: Theme.ink
            font.pixelSize: 12
            font.family: "Consolas, Menlo, monospace"
        }

        // .key-input: 28px wide, 18px tall, centered, 13px mono, single char
        TextInput {
            id: keyInput
            visible: root.editable
            anchors.centerIn: parent
            text: root.keyChar
            color: Theme.ink
            font.pixelSize: 13
            font.family: "Consolas, Menlo, monospace"
            horizontalAlignment: TextInput.AlignHCenter
            maximumLength: 1
            selectByMouse: false
            // capitalize single char to match "0-9 / A-Z"
            validator: RegularExpressionValidator {
                regularExpression: /[0-9A-Za-z]?/
            }
            onTextChanged: {
                var c = text.toUpperCase()
                if (c !== text) text = c
                root.keyCharEdited(c)
            }
            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.IBeamCursor
                onClicked: keyInput.forceActiveFocus()
            }
        }
    }
}
