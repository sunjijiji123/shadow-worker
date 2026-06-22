// SystemPage.qml - system page (v2).
// 6 cards: About / Launch at Startup / MCP Service / Software Update /
// Data Management / Config File Path.
// Matches ui-wireframe-v2.html .main-view[data-view="system"].

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    // ---- local state (will bind to a viewModel later) ----
    property bool launchAtStartup: true
    property bool checkUpdateOnStartup: true
    property bool checkUpdateDaily: true
    property string activeMcpTab: "claude"   // claude | cursor | raw

    // ---- MCP tools list ----
    property var mcpTools: [
        { name: "get_worklog",    desc: qsTr("Get the worklog (segments + events) for a given date, for daily/weekly reports") },
        { name: "get_summary",    desc: qsTr("Aggregate active minutes by category or app for a given day") },
        { name: "search_events",  desc: qsTr("Search voice / VLM / screenshot events by keyword") },
        { name: "list_apps",      desc: qsTr("List whitelisted apps and today's active minutes") }
    ]

    // ---- MCP config snippets (per tab) ----
    // mcpExePath / mcpReady are injected from main.cpp (resolved against the
    // actual backend exe location). Empty string means "not found".
    readonly property string mcpResolvedExePath: mcpExePath !== undefined ? mcpExePath : ""
    readonly property bool mcpResolvedReady: mcpReady !== undefined ? mcpReady : false
    readonly property string mcpClaudeConfig: {
        var p = root.mcpResolvedExePath.replace(/\\/g, "\\\\")
        return '{\n  "mcpServers": {\n    "shadow-worker": {\n      "command": "' + p + '",\n      "args": ["--mcp"]\n    }\n  }\n}'
    }
    readonly property string mcpCursorConfig: {
        var p = root.mcpResolvedExePath.replace(/\\/g, "\\\\")
        return '{\n  "mcp.servers": {\n    "shadow-worker": {\n      "command": "' + p + '",\n      "args": ["--mcp"]\n    }\n  }\n}'
    }
    readonly property string mcpRawConfig: {
        var p = root.mcpResolvedExePath.replace(/\\/g, "\\\\")
        return '{\n  "command": "' + p + '",\n  "args": ["--mcp"]\n}'
    }
    function currentMcpConfig() {
        if (activeMcpTab === "cursor") return mcpCursorConfig
        if (activeMcpTab === "raw") return mcpRawConfig
        return mcpClaudeConfig
    }

    function copyMcpConfig() {
        // QML provides clipboard via Qt's ApplicationWindow or TextField; use
        // the global clipboard through a transient hidden TextInput.
        copySink.text = currentMcpConfig()
        copySink.selectAll()
        copySink.copy()
        var win = ApplicationWindow.window
        if (win && win.toast) win.toast(qsTr("Config copied to clipboard"))
    }

    // hidden TextInput used only as a clipboard conduit
    TextInput {
        id: copySink
        visible: false
        readOnly: true
    }

    Flickable {
        anchors.fill: parent
        anchors.margins: Theme.contentPadding
        contentWidth: width
        contentHeight: contentCol.implicitHeight
        flickableDirection: Flickable.VerticalFlick
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

        ColumnLayout {
            id: contentCol
            width: parent.width
            spacing: Theme.cardSpacing

            Text {
                text: qsTr("System")
                color: Theme.ink
                font.pixelSize: Theme.fontTitle
                font.weight: Font.DemiBold
            }

            // ============================================================
            // Card 1: About
            // ============================================================
            Card {
                Layout.fillWidth: true
                title: qsTr("About")
                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 4
                    Text {
                        text: "Shadow Worker v0.2.0"
                        color: Theme.muted
                        font.pixelSize: 14
                    }
                    Text {
                        text: "git@github.com:sunjijiji123/shadow-worker.git"
                        color: Theme.muted
                        font.pixelSize: 12
                    }
                }
            }

            // ============================================================
            // Card 2: Launch at Startup
            // ============================================================
            Card {
                Layout.fillWidth: true
                title: qsTr("Launch at Startup")
                description: qsTr("Auto-start to tray after Windows login.")
                headerExtra: [
                    Toggle {
                        checked: launchAtStartup
                        onToggled: launchAtStartup = checked
                    }
                ]
            }

            // ============================================================
            // Card 3: MCP Service
            // ============================================================
            Card {
                Layout.fillWidth: true
                title: qsTr("MCP Service")
                description: qsTr("Exposes worklog query tools to local AI agents via stdio.")
                headerExtra: [
                    RowLayout {
                        spacing: 6
                        // status light (8px dot): accent when the backend exe is
                        // found (MCP is usable on demand), muted when missing.
                        Rectangle {
                            width: 8; height: 8; radius: 4
                            color: mcpResolvedReady ? Theme.accent : Theme.muted
                        }
                        Text {
                            text: mcpResolvedReady ? qsTr("Ready") : qsTr("Backend not found")
                            color: Theme.ink
                            font.pixelSize: 13
                        }
                    }
                ]

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 0

                    // backend exe not found: show the resolved/expected path so
                    // the user knows what to build or where to put the binary.
                    Text {
                        Layout.fillWidth: true
                        visible: !mcpResolvedReady
                        text: qsTr("Backend executable (shadow-worker.exe) not found. Build the backend first; the path below is what the client expects. MCP will start on demand when an agent spawns it.")
                        color: Theme.muted
                        font.pixelSize: 12
                        wrapMode: Text.WordWrap
                    }

                    // resolved exe path (read-only display)
                    TextField {
                        Layout.fillWidth: true
                        Layout.topMargin: 4
                        label: qsTr("Backend executable")
                        text: mcpResolvedExePath !== "" ? mcpResolvedExePath : qsTr("(not found)")
                        readOnly: true
                    }

                    // ---- available tools ----
                    Text {
                        Layout.topMargin: 20
                        text: qsTr("Available Tools")
                        color: Theme.ink
                        font.pixelSize: 13
                        font.weight: Font.DemiBold
                    }

                    ColumnLayout {
                        Layout.fillWidth: true
                        Layout.topMargin: 8
                        spacing: 8

                        Repeater {
                            model: mcpTools
                            delegate: Rectangle {
                                required property var modelData
                                Layout.fillWidth: true
                                // .mcp-tool: bg, rule border, radius 8, padding 10x12
                                color: Theme.bg
                                border.color: Theme.rule
                                border.width: 1
                                radius: 8
                                height: toolRow.implicitHeight + 20

                                RowLayout {
                                    id: toolRow
                                    anchors.fill: parent
                                    anchors.margins: 10
                                    spacing: 10

                                    Text {
                                        // .mcp-tool-name: 13px bold accent monospace, nowrap
                                        text: modelData.name
                                        color: Theme.accent
                                        font.pixelSize: 13
                                        font.weight: Font.DemiBold
                                        font.family: "Consolas, Menlo, monospace"
                                        Layout.alignment: Qt.AlignTop
                                    }
                                    Text {
                                        text: modelData.desc
                                        color: Theme.muted
                                        font.pixelSize: 12
                                        wrapMode: Text.WordWrap
                                        Layout.fillWidth: true
                                    }
                                }
                            }
                        }
                    }

                    // ---- connection config ----
                    Text {
                        Layout.topMargin: 20
                        text: qsTr("Connection Config")
                        color: Theme.ink
                        font.pixelSize: 13
                        font.weight: Font.DemiBold
                    }
                    Text {
                        Layout.topMargin: 4
                        text: qsTr("Copy the config below and paste it into your tool's MCP config file.")
                        color: Theme.muted
                        font.pixelSize: 13
                        wrapMode: Text.WordWrap
                        Layout.fillWidth: true
                    }

                    // config tabs
                    Row {
                        Layout.topMargin: 8
                        spacing: 6

                        Repeater {
                            model: [
                                { key: "claude", label: "Claude Desktop" },
                                { key: "cursor", label: "Cursor" },
                                { key: "raw",    label: qsTr("Raw JSON") }
                            ]
                            delegate: Rectangle {
                                required property var modelData
                                property bool isActive: root.activeMcpTab === modelData.key
                                // .mcp-config-tab: padding 5x12, 12px, radius 5, bg/rule border
                                height: 28
                                width: tabTxt.implicitWidth + 24
                                radius: 5
                                color: isActive ? Qt.rgba(16/255, 185/255, 129/255, 0.08) : Theme.bg
                                border.color: isActive ? Theme.accent : Theme.rule
                                border.width: 1

                                Text {
                                    id: tabTxt
                                    anchors.centerIn: parent
                                    text: modelData.label
                                    color: isActive ? Theme.accent : Theme.muted
                                    font.pixelSize: 12
                                }
                                MouseArea {
                                    anchors.fill: parent
                                    cursorShape: Qt.PointingHandCursor
                                    onClicked: root.activeMcpTab = modelData.key
                                }
                            }
                        }
                    }

                    // config code box (.mcp-config-box: #0d0d0f bg, rule border, radius 8)
                    Rectangle {
                        Layout.fillWidth: true
                        Layout.topMargin: 8
                        color: "#0d0d0f"
                        border.color: Theme.rule
                        border.width: 1
                        radius: 8
                        implicitHeight: 140

                        // copy button (top-right)
                        Rectangle {
                            id: copyBtn
                            anchors.top: parent.top
                            anchors.right: parent.right
                            anchors.topMargin: 8
                            anchors.rightMargin: 8
                            // .mcp-config-copy: padding 4x10, 12px, bg2/rule, radius 5
                            height: 24
                            width: copyBtnTxt.implicitWidth + 20
                            radius: 5
                            color: Theme.bg2
                            border.color: copyMa.containsMouse ? Theme.accent : Theme.rule
                            border.width: 1
                            z: 2

                            Text {
                                id: copyBtnTxt
                                anchors.centerIn: parent
                                text: qsTr("Copy")
                                color: copyMa.containsMouse ? Theme.accent : Theme.muted
                                font.pixelSize: 12
                            }
                            MouseArea {
                                id: copyMa
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                hoverEnabled: true
                                onClicked: root.copyMcpConfig()
                            }
                        }

                        // code text (pre.mcp-config-code: padding 14x16, 12px mono)
                        Flickable {
                            anchors.fill: parent
                            anchors.margins: 1
                            contentWidth: codeTxt.implicitWidth
                            contentHeight: codeTxt.implicitHeight
                            flickableDirection: Flickable.HorizontalFlick
                            clip: true
                            boundsBehavior: Flickable.StopAtBounds

                            Text {
                                id: codeTxt
                                x: 16
                                y: 14
                                text: root.currentMcpConfig()
                                color: "#e4e4e7"
                                font.pixelSize: 12
                                font.family: "Consolas, Menlo, monospace"
                                textFormat: Text.PlainText
                            }
                        }
                    }
                }
            }

            // ============================================================
            // Card 4: Software Update
            // ============================================================
            Card {
                Layout.fillWidth: true
                title: qsTr("Software Update")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 12

                    // .update-row: title/version/badge on left, link buttons on right
                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 16

                        ColumnLayout {
                            spacing: 6
                            Text {
                                text: qsTr("Current version v0.2.0 · last checked 2026-06-18")
                                color: Theme.muted
                                font.pixelSize: 13
                            }
                            Badge {
                                text: qsTr("Update service unavailable")
                                kind: "warn"
                            }
                        }

                        Item { Layout.fillWidth: true }

                        // .update-links: gap 10, wrap
                        Row {
                            spacing: 10
                            Button {
                                text: qsTr("Website")
                                kind: "ghost"
                                onClicked: {
                                    var win = ApplicationWindow.window
                                    if (win && win.toast) win.toast(qsTr("Opening website..."))
                                }
                            }
                            Button {
                                text: "GitHub"
                                kind: "ghost"
                            }
                            Button {
                                text: qsTr("Changelog")
                                kind: "ghost"
                            }
                            Button {
                                text: qsTr("Check for Updates")
                                kind: "primary"
                                onClicked: {
                                    var win = ApplicationWindow.window
                                    if (win && win.toast) win.toast(qsTr("Update service unavailable"), "warning")
                                }
                            }
                        }
                    }

                    Rectangle { Layout.fillWidth: true; height: 1; color: Theme.rule }

                    // startup check toggle
                    RowLayout {
                        Layout.fillWidth: true
                        ColumnLayout {
                            spacing: 2
                            Text {
                                text: qsTr("Check on startup")
                                color: Theme.ink
                                font.pixelSize: 13
                            }
                            Text {
                                text: qsTr("Check for a new version once on each launch")
                                color: Theme.muted
                                font.pixelSize: 12
                            }
                        }
                        Item { Layout.fillWidth: true }
                        Toggle {
                            checked: checkUpdateOnStartup
                            onToggled: checkUpdateOnStartup = checked
                        }
                    }

                    // daily check toggle
                    RowLayout {
                        Layout.fillWidth: true
                        ColumnLayout {
                            spacing: 2
                            Text {
                                text: qsTr("Check daily")
                                color: Theme.ink
                                font.pixelSize: 13
                            }
                            Text {
                                text: qsTr("Background check once a day; stays silent when the service is down")
                                color: Theme.muted
                                font.pixelSize: 12
                            }
                        }
                        Item { Layout.fillWidth: true }
                        Toggle {
                            checked: checkUpdateDaily
                            onToggled: checkUpdateDaily = checked
                        }
                    }

                    // update server url
                    TextField {
                        Layout.fillWidth: true
                        label: qsTr("Update Server URL")
                        text: "https://updates.shadow-worker.example"
                    }
                }
            }

            // ============================================================
            // Card 5: Data Management
            // ============================================================
            Card {
                Layout.fillWidth: true
                title: qsTr("Data Management")
                description: qsTr("Local SQLite database and screenshot cache.")

                Row {
                    spacing: 12
                    Layout.topMargin: 4
                    Button {
                        text: qsTr("Open Data Directory")
                        kind: "ghost"
                        onClicked: {
                            var win = ApplicationWindow.window
                            if (win && win.toast) win.toast(qsTr("Opening data directory..."))
                        }
                    }
                    Button {
                        text: qsTr("Clear All Records")
                        kind: "danger"
                        onClicked: clearDataConfirm.open()
                    }
                }
            }

            // ============================================================
            // Card 6: Config File Path
            // ============================================================
            Card {
                Layout.fillWidth: true
                title: qsTr("Config File Path")
                TextField {
                    Layout.fillWidth: true
                    text: "%APPDATA%\\shadow-worker\\config.yaml"
                    readOnly: true
                }
            }

            // bottom spacer
            Item { Layout.fillWidth: true; Layout.preferredHeight: 8 }
        }
    }

    // clear-data confirm (destructive)
    ConfirmDialog {
        id: clearDataConfirm
        parent: Overlay.overlay
        heading: qsTr("Clear All Records")
        message: qsTr("This permanently deletes the local database and screenshot cache. This cannot be undone.")
        confirmText: qsTr("Clear Data")
        destructive: true
        onConfirmed: {
            var win = ApplicationWindow.window
            if (win && win.toast) win.toast(qsTr("All records cleared"))
        }
    }
}
