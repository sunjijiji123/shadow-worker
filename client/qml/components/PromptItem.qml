// PromptItem.qml - one row in the personal prompt list (HTML .prompt-item).
// Layout: header (name input + shortcut key-caps + delete) + textarea + hint.
// Styled to match the inline <style> block in ui-wireframe-v2.html.

import QtQuick
import QtQuick.Layouts
import ShadowWorker

// .prompt-item: bg fill, rule border, radius 8, padding 12, gap 10
Rectangle {
    id: root

    // ---- API ----
    property string name: ""
    property string keyChar: ""        // single char ("1", "A", ...)
    property string text: ""           // prompt content
    property string prefixLabel: "Ctrl" // the global prefix key label

    signal nameEdited(string newName)
    signal keyEdited(string newKey)
    signal textEdited(string newText)
    signal deleteRequested()

    color: Theme.bg
    border.color: Theme.rule
    border.width: 1
    radius: 8
    implicitHeight: layout.implicitHeight + 24   // padding 12 * 2

    ColumnLayout {
        id: layout
        anchors.fill: parent
        anchors.margins: 12
        spacing: 10

        // ---- header: name (flex) + shortcut caps + delete ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 10

            // .prompt-name: transparent, no border, 14px bold ink (flex: 1)
            TextField {
                id: nameInput
                Layout.fillWidth: true
                text: root.name
                placeholder: qsTr("Prompt name")
                nameMode: true
                onTextEdited: root.nameEdited(newText)
            }

            // .prompt-shortcut: prefix cap + "+" + key cap
            RowLayout {
                spacing: 6
                Layout.alignment: Qt.AlignVCenter

                // prefix key-cap (fixed label from global prefix)
                KeyCap {
                    text: root.prefixLabel
                }

                Text {
                    text: "+"
                    color: Theme.muted
                    font.pixelSize: 12
                    Layout.alignment: Qt.AlignVCenter
                }

                // key-input cap (editable single char)
                KeyCap {
                    editable: true
                    keyChar: root.keyChar
                    onKeyCharEdited: root.keyEdited(newKey)
                }
            }

            // .delete-prompt: danger small button
            Button {
                text: qsTr("Delete")
                kind: "danger"
                small: true
                onClicked: root.deleteRequested()
            }
        }

        // .prompt-textarea: bg2, rule border, radius 6, min-height 64
        TextArea {
            Layout.fillWidth: true
            text: root.text
            placeholder: qsTr("Enter the full prompt content...")
            minHeight: 64
            frameColor: Theme.bg2    // .prompt-textarea uses bg2 (not bg)
        }

        // .prompt-hint: 12px muted, combo in accent green
        Text {
            Layout.fillWidth: true
            text: qsTr("Press %1 to inject this prompt").arg(
                  "<b style='color:" + Theme.accent + "'>" + root.prefixLabel + " + " + root.keyChar + "</b>")
            textFormat: Text.RichText
            color: Theme.muted
            font.pixelSize: 12
            wrapMode: Text.WordWrap
        }
    }
}
