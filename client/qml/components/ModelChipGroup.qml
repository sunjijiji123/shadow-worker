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

    signal chipClicked(string key)
    signal addClicked()

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

                    // close button (deletable chips)
                    Rectangle {
                        visible: modelData.deletable !== false
                        width: 14; height: 14; radius: 7
                        color: closeMa.containsMouse ? Qt.rgba(239/255,68/255,68/255,0.2) : "transparent"
                        anchors.verticalCenter: parent.verticalCenter
                        Text {
                            anchors.centerIn: parent
                            text: "\u00D7"   // ×
                            color: closeMa.containsMouse ? Theme.danger : Theme.muted
                            font.pixelSize: 11
                        }
                        MouseArea {
                            id: closeMa
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {}  // TODO: emit delete signal
                        }
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: root.chipClicked(modelData.key)
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
