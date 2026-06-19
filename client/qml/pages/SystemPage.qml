// SystemPage.qml - system page skeleton (v2).
// About / Autostart / MCP Service / Update / Data Management / Config Path.
// M2: basic card structure; M3/M4 add MCP config display + update check.

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 20
        spacing: 16

        Text {
            text: qsTr("System")
            color: Theme.ink
            font.pixelSize: Theme.fontTitle
            font.weight: Font.DemiBold
        }

        Card {
            Layout.fillWidth: true
            title: qsTr("About")
            PlaceholderLabel { text: "Shadow Worker v0.2.0" }
        }

        Card {
            Layout.fillWidth: true
            title: qsTr("Launch at Startup")
            description: qsTr("Auto-start to tray after Windows login")
            RowLayout {
                Layout.fillWidth: true
                Item { Layout.fillWidth: true }
                Toggle { checked: true }
            }
        }

        Card {
            Layout.fillWidth: true
            title: qsTr("MCP Service")
            description: qsTr("Exposes worklog query tools to local AI agents via stdio")
            PlaceholderLabel { text: qsTr("M3: status light + restart + 4 tools + 3-tab config copy") }
        }

        Card {
            Layout.fillWidth: true
            title: qsTr("Software Update")
            PlaceholderLabel { text: qsTr("M3: version + check update + startup/daily toggle") }
        }

        Card {
            Layout.fillWidth: true
            title: qsTr("Data Management")
            description: qsTr("Local SQLite database and screenshot cache")
            RowLayout {
                Layout.fillWidth: true
                spacing: 12
                Button { text: qsTr("Open Data Folder") }
                Button { text: qsTr("Clear All Records"); kind: "danger" }
            }
        }

        Card {
            Layout.fillWidth: true
            title: qsTr("Config File Path")
            PlaceholderLabel { text: "%APPDATA%\\shadow-worker\\config.yaml" }
        }

        Item { Layout.fillHeight: true }
    }
}
