// TextField.qml - labeled input field (HTML .field + .label + .input).
// Supports password mode with show/hide toggle (for API Key).
// Uses raw TextInput + Rectangle background (avoids name clash with QtQuick.Controls TextField).

import QtQuick
import QtQuick.Layouts
import ShadowWorker

ColumnLayout {
    id: root

    property string label: ""
    property string text: ""
    property string placeholder: ""
    property bool readOnly: false
    property bool isPassword: false
    property bool spanFull: false   // grid-column: 1/-1 in HTML two-col

    signal textEdited(string newText)

    spacing: 6
    Layout.fillWidth: true

    Text {
        visible: root.label !== ""
        text: root.label
        color: Theme.muted
        font.pixelSize: 12
    }

    // input row (input + optional show/hide button for password)
    RowLayout {
        Layout.fillWidth: true
        spacing: 8

        // input background
        Rectangle {
            Layout.fillWidth: true
            height: 36
            color: Theme.bg
            border.color: inputBody.activeFocus ? Theme.accent : Theme.rule
            border.width: 1
            radius: 6

            TextInput {
                id: inputBody
                anchors.fill: parent
                anchors.leftMargin: 10
                anchors.rightMargin: 10
                anchors.verticalCenter: parent.verticalCenter
                verticalAlignment: TextInput.AlignVCenter
                text: root.text
                readOnly: root.readOnly
                echoMode: root.isPassword && !pwToggle.pwShown ? TextInput.Password : TextInput.Normal
                color: Theme.ink
                font.pixelSize: 13
                selectByMouse: true
                onTextEdited: root.textEdited(inputBody.text)
            }
        }

        // show/hide button for password fields
        Button {
            id: pwToggle
            visible: root.isPassword
            text: pwShown ? qsTr("Hide") : qsTr("Show")
            kind: "ghost"
            small: true
            property bool pwShown: false
            onClicked: pwShown = !pwShown
        }
    }
}
