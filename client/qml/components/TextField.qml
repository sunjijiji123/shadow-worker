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
    // .prompt-name mode: borderless, transparent, 14px bold (looks like text,
    // not a form input). Used inside PromptItem's header.
    property bool nameMode: false
    // captureMode: when true the field becomes a key-capture target — focus it,
    // press any key, and it records that key (read-only, no manual typing).
    // Used by hotkey configuration.
    property bool captureMode: false

    signal textEdited(string newText)
    // emitted only in captureMode, with a normalized key name (e.g. "R", "F9",
    // "Space", "Esc"). Empty string = modifier-only / ignored press.
    signal keyPressed(string keyName)

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
                // captureMode forces read-only: the value comes from a key press,
                // not from typing.
                readOnly: root.readOnly || root.captureMode
                echoMode: root.isPassword && !pwToggle.pwShown ? TextInput.Password : TextInput.Normal
                color: Theme.ink
                // .prompt-name: 14px DemiBold; .input: 13px regular
                font.pixelSize: root.nameMode ? 14 : 13
                font.weight: root.nameMode ? Font.DemiBold : Font.Normal
                // captureMode shows the cursor still so it looks focusable, but
                // selection is meaningless there.
                selectByMouse: !root.captureMode
                onTextEdited: root.textEdited(inputBody.text)

                // ---- key capture (captureMode only) ----
                // On any key press, translate the Qt key to a normalized name
                // matching the GlobalHotkey registration format, and emit it.
                // Pure-modifier presses (Ctrl/Shift/Alt alone) are ignored.
                Keys.onPressed: function(event) {
                    if (!root.captureMode) return
                    var name = keyToName(event.key, event.text)
                    if (name !== "") {
                        root.text = name
                        root.keyPressed(name)
                    }
                    event.accepted = true   // never let it reach text input
                }

                // map a Qt.Key + text to a hotkey key name. Returns "" for
                // modifier-only / non-mappable presses.
                function keyToName(k, txt) {
                    // function keys
                    if (k >= Qt.Key_F1 && k <= Qt.Key_F24)
                        return "F" + (k - Qt.Key_F1 + 1)
                    // digits 0-9
                    if (k >= Qt.Key_0 && k <= Qt.Key_9)
                        return String.fromCharCode(k)
                    // letters A-Z (uppercase)
                    if (k >= Qt.Key_A && k <= Qt.Key_Z)
                        return String.fromCharCode(k)
                    // named special keys
                    var named = {}
                    named[Qt.Key_Space] = "Space"
                    named[Qt.Key_Return] = "Enter"
                    named[Qt.Key_Enter] = "Enter"
                    named[Qt.Key_Tab] = "Tab"
                    named[Qt.Key_Escape] = "Esc"
                    named[Qt.Key_Insert] = "Insert"
                    named[Qt.Key_Delete] = "Delete"
                    named[Qt.Key_Home] = "Home"
                    named[Qt.Key_End] = "End"
                    named[Qt.Key_PageUp] = "PageUp"
                    named[Qt.Key_PageDown] = "PageDown"
                    named[Qt.Key_Left] = "Left"
                    named[Qt.Key_Right] = "Right"
                    named[Qt.Key_Up] = "Up"
                    named[Qt.Key_Down] = "Down"
                    named[Qt.Key_Print] = "PrtScn"
                    if (named[k]) return named[k]
                    // modifier-only presses -> ignore
                    return ""
                }
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
