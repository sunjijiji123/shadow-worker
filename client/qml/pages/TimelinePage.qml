// TimelinePage.qml - timeline page skeleton (v2).
// M2: page title + date picker placeholder + placeholders.
// M3 TODO: calendar popup / timeline track segments / worklog+events dual tabs
//          (needs VM QueryTimeline).

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var viewModel: null

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 20
        spacing: 16

        // ---- title bar + date picker (M2 placeholder) ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 8

            Text {
                text: qsTr("Timeline")
                color: Theme.ink
                font.pixelSize: Theme.fontTitle
                font.weight: Font.DemiBold
            }
            Item { Layout.fillWidth: true }

            // date picker (M3 adds full calendar popup; M2 static)
            Rectangle {
                color: Theme.bg3
                border.color: Theme.rule
                border.width: 1
                radius: 8
                implicitWidth: 200
                implicitHeight: 32
                Layout.alignment: Qt.AlignVCenter

                RowLayout {
                    anchors.centerIn: parent
                    spacing: 8
                    Text { text: "<"; color: Theme.muted }
                    Text {
                        text: Qt.formatDate(new Date(), "yyyy-MM-dd")
                        color: Theme.ink
                        font.pixelSize: Theme.fontSmall
                    }
                    Text { text: ">"; color: Theme.muted }
                }
            }

            Button { text: qsTr("Today"); small: true }
        }

        // ---- placeholders: M3 timeline track + worklog/events ----
        Card {
            Layout.fillWidth: true
            title: qsTr("Day View")
            description: qsTr("M3: single-focus timeline track (category color blocks) + ruler")
            PlaceholderLabel {
                text: qsTr("Pending M3 (needs VM QueryTimeline; backend collection.proto has the RPC)")
            }
        }

        Card {
            Layout.fillWidth: true
            title: qsTr("Worklog / Events")
            description: qsTr("M3: dual tabs + category/type filter chips")
            PlaceholderLabel { text: qsTr("Pending M3") }
        }

        Item { Layout.fillHeight: true }
    }
}
