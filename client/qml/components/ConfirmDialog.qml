// ConfirmDialog.qml - reusable confirmation modal (mirrors AddModelDialog style).
// Usage:
//   ConfirmDialog {
//       title: qsTr("Delete Model")
//       message: qsTr("Remove this model? ...")
//       confirmText: qsTr("Delete")
//       destructive: true
//       onConfirmed: doDelete()
//   }
// Caller controls open()/close().

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Dialog {
    id: root

    modal: true
    closePolicy: Dialog.CloseOnEscape
    anchors.centerIn: parent
    width: 420
    padding: 20
    topPadding: 20
    bottomPadding: 20
    leftPadding: 20
    rightPadding: 20

    // ---- API ----
    // NOTE: "title" is a FINAL property on QtQuick.Controls Dialog, so we use
    // "heading" instead to avoid a QML override error.
    property string heading: ""
    property string message: ""
    property string confirmText: qsTr("Confirm")
    property string cancelText: qsTr("Cancel")
    // destructive=true -> confirm button uses danger styling (red)
    property bool destructive: false

    signal confirmed()
    signal cancelled()

    background: Rectangle {
        color: Theme.bg3
        border.color: Theme.rule
        border.width: 1
        radius: 12
    }

    contentItem: ColumnLayout {
        spacing: 12

        Text {
            visible: root.heading !== ""
            text: root.heading
            color: Theme.ink
            font.pixelSize: 16
            font.weight: Font.DemiBold
            Layout.fillWidth: true
            wrapMode: Text.WordWrap
        }

        Text {
            text: root.message
            color: Theme.muted
            font.pixelSize: 13
            Layout.fillWidth: true
            wrapMode: Text.WordWrap
        }

        // footer: cancel + confirm
        RowLayout {
            Layout.fillWidth: true
            Layout.topMargin: 8
            spacing: 10

            Item { Layout.fillWidth: true }

            Button {
                text: root.cancelText
                kind: "ghost"
                onClicked: {
                    root.cancelled()
                    root.close()
                }
            }
            Button {
                text: root.confirmText
                kind: root.destructive ? "danger" : "primary"
                onClicked: {
                    root.confirmed()
                    root.close()
                }
            }
        }
    }
}
