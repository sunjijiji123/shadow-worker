// ModelChipGroup.qml - pill-shaped model selector (HTML .model-chip-group).
// chips: [{key, label, type, deletable}] + a "+ Add" chip at the end.
// activeKey: which chip is selected. signal chipClicked(key).

import QtQuick
import QtQuick.Layouts
import ShadowWorker

ColumnLayout {
    id: root

    property var chips: []
    property string activeKey: ""
    // when only one chip remains, its × is disabled to avoid an empty list
    // (the form below has no model to bind to). Emit closeBlocked for feedback.
    readonly property bool canDelete: chips.length > 1

    signal chipClicked(string key)
    signal addClicked()
    signal chipClosed(string key)
    signal closeBlocked()

    spacing: 0

    Row {
        id: chipRow
        spacing: 8
        layoutDirection: Qt.LeftToRight

        Repeater {
            model: root.chips

            delegate: Rectangle {
                required property var modelData
                property bool isActive: root.activeKey === modelData.key

                height: 32
                width: chipContent.implicitWidth + 24
                radius: 16
                color: isActive ? Theme.accentBg2 : Theme.bg
                border.color: isActive ? Theme.accent : Theme.rule
                border.width: 1

                Behavior on border.color { ColorAnimation { duration: 150 } }
                Behavior on color { ColorAnimation { duration: 150 } }

                Row {
                    id: chipContent
                    anchors.centerIn: parent
                    spacing: 6

                    Text {
                        text: modelData.label
                        color: isActive ? Theme.ink : Theme.muted
                        font.pixelSize: 13
                        anchors.verticalCenter: parent.verticalCenter
                    }

                    // spacer that reserves room on the right of the label for the
                    // close button (which is a sibling rendered above selectMa).
                    // Does NOT draw the × glyph itself.
                    Item {
                        visible: modelData.deletable !== false
                        width: 16; height: 16
                        anchors.verticalCenter: parent.verticalCenter
                    }
                }

                // chip select hit area (covers whole chip; sits BELOW close button in z)
                MouseArea {
                    id: selectMa
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.chipClicked(modelData.key)
                }

                // close hit area - direct child of the chip Rectangle, with higher z
                // so it reliably receives hover/click instead of the select MouseArea.
                // Disabled (dim ×, no hover highlight, no delete) when this is the
                // last remaining chip — see root.canDelete.
                Rectangle {
                    id: closeBtn
                    visible: modelData.deletable !== false
                    anchors.right: parent.right
                    anchors.rightMargin: 8
                    anchors.verticalCenter: parent.verticalCenter
                    width: 16; height: 16; radius: 8
                    // only show red hover bg when deletion is allowed
                    color: (root.canDelete && closeMa.containsMouse)
                           ? Qt.rgba(239/255,68/255,68/255,0.2) : "transparent"
                    z: 10   // above selectMa
                    Behavior on color { ColorAnimation { duration: 120 } }

                    Text {
                        anchors.centerIn: parent
                        text: "\u00D7"   // ×
                        color: root.canDelete
                               ? (closeMa.containsMouse ? Theme.danger : Theme.muted)
                               : Qt.rgba(0.4, 0.4, 0.4, 1)   // dimmed/disabled
                        font.pixelSize: 12
                        Behavior on color { ColorAnimation { duration: 120 } }
                    }

                    MouseArea {
                        id: closeMa
                        anchors.fill: parent
                        cursorShape: root.canDelete ? Qt.PointingHandCursor : Qt.ArrowCursor
                        hoverEnabled: true
                        onClicked: {
                            if (root.canDelete) root.chipClosed(modelData.key)
                            else root.closeBlocked()
                        }
                    }
                }
            }
        }

        // "+ Add" chip (dashed border)
        Rectangle {
            height: 32
            width: addTxt.implicitWidth + 24
            radius: 16
            color: "transparent"
            border.color: Theme.rule
            border.width: 1

            Rectangle {
                // dashed effect via semi-transparent overlay pattern (simplified: just dashed style hint)
                anchors.fill: parent
                color: "transparent"
                border.color: addMa.containsMouse ? Theme.accent : Theme.rule
                border.width: 1
                radius: 16
            }

            Text {
                id: addTxt
                anchors.centerIn: parent
                text: qsTr("+ Add Model")
                color: addMa.containsMouse ? Theme.accent : Theme.muted
                font.pixelSize: 13
            }

            MouseArea {
                id: addMa
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                hoverEnabled: true
                onClicked: root.addClicked()
            }
        }
    }
}
