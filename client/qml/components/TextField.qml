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
    // .prompt-name mode: borderless, transparent bg, 14px bold (looks like text,
    // not a form input). Used inside PromptItem's header.
    property bool nameMode: false

    signal textEdited(string newText)

    // button width sized to the WIDER of "Show"/"Hide" + padding (matches
    // Button.qml's +28) so the button keeps a constant width when toggling.
    readonly property int pwBtnWidth: Math.max(pwShowLbl.implicitWidth, pwHideLbl.implicitWidth) + 28

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
            // nameMode (.prompt-name): transparent, borderless; else form input.
            color: root.nameMode ? "transparent" : Theme.bg
            border.color: root.nameMode ? "transparent"
                                      : (inputBody.activeFocus ? Theme.accent : Theme.rule)
            border.width: root.nameMode ? 0 : 1
            radius: 6

            TextInput {
                id: inputBody
                anchors.fill: parent
                anchors.leftMargin: root.nameMode ? 0 : 10
                anchors.rightMargin: root.nameMode ? 0 : 10
                anchors.verticalCenter: parent.verticalCenter
                verticalAlignment: TextInput.AlignVCenter
                text: root.text
                readOnly: root.readOnly
                echoMode: root.isPassword && !pwToggle.pwShown ? TextInput.Password : TextInput.Normal
                color: Theme.ink
                // .prompt-name: 14px DemiBold; .input: 13px regular
                font.pixelSize: root.nameMode ? 14 : 13
                font.weight: root.nameMode ? Font.DemiBold : Font.Normal
                selectByMouse: true
                onTextEdited: root.textEdited(inputBody.text)
            }
        }

        // show/hide button for password fields.
        // - height matches the input (36px), bottom-aligned to the input row
        // - width is FIXED (explicit, not derived from the live label) so the
        //   button never shifts between "Show" and "Hide".
        Button {
            id: pwToggle
            visible: root.isPassword
            text: pwShown ? qsTr("Hide") : qsTr("Show")
            kind: "ghost"
            // explicit Layout size beats Button.qml's implicitWidth/implicitHeight
            Layout.preferredWidth: root.pwBtnWidth
            Layout.preferredHeight: 36
            Layout.minimumWidth: root.pwBtnWidth
            Layout.maximumWidth: root.pwBtnWidth
            Layout.fillWidth: false
            Layout.fillHeight: false
            Layout.alignment: Qt.AlignBottom
            property bool pwShown: false
            onClicked: pwShown = !pwShown
        }
    }

    // invisible measuring texts used to compute pwBtnWidth (max of the two).
    Text { id: pwShowLbl; visible: false; font.pixelSize: Theme.fontSmall; text: qsTr("Show") }
    Text { id: pwHideLbl; visible: false; font.pixelSize: Theme.fontSmall; text: qsTr("Hide") }
}
