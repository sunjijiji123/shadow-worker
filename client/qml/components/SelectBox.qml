// SelectBox.qml - labeled dropdown select (HTML .select-wrap + .select-options).
// options: list of strings. currentIndex: selected index.

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

ColumnLayout {
    id: root

    property string label: ""
    property var options: []
    property int currentIndex: 0
    property bool spanFull: false

    signal selected(int index, string value)

    spacing: 6
    Layout.fillWidth: true

    Text {
        visible: root.label !== ""
        text: root.label
        color: Theme.muted
        font.pixelSize: 12
    }

    // trigger + dropdown (use Popup for correct z-order)
    Item {
        id: selectWrap
        Layout.fillWidth: true
        height: 36
        width: parent.width

        property bool open: false

        // trigger
        Rectangle {
            id: trigger
            anchors.fill: parent
            color: Theme.bg
            border.color: selectWrap.open ? Theme.accent : Theme.rule
            border.width: 1
            radius: 6

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 10
                anchors.rightMargin: 10

                Text {
                    text: root.options.length > 0 ? root.options[root.currentIndex] : ""
                    color: Theme.ink
                    font.pixelSize: 13
                    Layout.fillWidth: true
                }
                Text {
                    text: "\u25BE"   // ▾
                    color: Theme.muted
                    font.pixelSize: 12
                }
            }

            MouseArea {
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                onClicked: selectWrap.open = !selectWrap.open
            }
        }

        // dropdown popup (Overlay layer)
        Popup {
            id: dropdown
            x: 0
            y: trigger.height + 4
            width: selectWrap.width
            padding: 0
            modal: false
            closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside
            visible: selectWrap.open
            onVisibleChanged: if (!visible) selectWrap.open = false

            background: Rectangle {
                color: Theme.bg2
                border.color: Theme.rule
                border.width: 1
                radius: 6
            }

            ColumnLayout {
                anchors.fill: parent
                spacing: 0

                Repeater {
                    model: root.options

                    delegate: Rectangle {
                        required property string modelData
                        required property int index
                        Layout.fillWidth: true
                        height: 34
                        color: optMa.containsMouse ? Theme.bg3
                             : (index === root.currentIndex ? Theme.accentBg : "transparent")

                        Text {
                            anchors.left: parent.left
                            anchors.leftMargin: 10
                            anchors.verticalCenter: parent.verticalCenter
                            text: modelData
                            color: index === root.currentIndex ? Theme.accent : Theme.ink
                            font.pixelSize: 13
                        }

                        MouseArea {
                            id: optMa
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            hoverEnabled: true
                            onClicked: {
                                root.currentIndex = index
                                root.selected(index, modelData)
                                selectWrap.open = false
                            }
                        }
                    }
                }
            }
        }
    }
}
