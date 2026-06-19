// main.qml - Shadow Worker main window (v2 rewrite)
// 180px sidebar (Overview/Timeline/Settings/System) + content view switch + global Toast.
// Source of truth: docs/ui-spec-v2.md section 2.
// All strings English; Chinese via Qt i18n (.ts/.qm).

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

ApplicationWindow {
    id: mainWindow
    visible: true
    width: 1200
    height: 720
    // Fixed size: no user resize (simplifies layout). DPI scaling handled by Qt6.
    minimumWidth: 1200
    maximumWidth: 1200
    minimumHeight: 720
    maximumHeight: 720
    title: qsTr("Shadow Worker")
    color: Theme.bg2

    property string currentView: "overview"

    RowLayout {
        anchors.fill: parent
        spacing: 0

        // ==================== Sidebar ====================
        Rectangle {
            id: sidebar
            Layout.fillHeight: true
            Layout.preferredWidth: Theme.sidebarWidth
            color: Theme.bg2
            border.color: Theme.rule
            border.width: 0

            // right divider line
            Rectangle {
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.bottom: parent.bottom
                width: 1
                color: Theme.rule
            }

            ColumnLayout {
                anchors.fill: parent
                anchors.topMargin: 16
                spacing: 0

                Repeater {
                    model: [
                        { view: "overview", label: qsTr("Overview"), icon: "M10 20v-6h4v6h5v-8h3L12 3 2 12h3v8z" },
                        { view: "timeline", label: qsTr("Timeline"), icon: "M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10 10-4.5 10-10S17.5 2 12 2zm4.2 14.2L11 13V7h2v5l4.5 2.7-.8 1.5z" },
                        { view: "settings", label: qsTr("Settings"), icon: "M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58a.49.49 0 0 0 .12-.61l-1.92-3.32a.488.488 0 0 0-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54a.484.484 0 0 0-.48-.41h-3.84a.484.484 0 0 0-.48.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96a.488.488 0 0 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.07.63-.07.94s.02.64.07.94l-2.03 1.58a.49.49 0 0 0-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.27.41.48.41h3.84c.24 0 .44-.17.48-.41l.36-2.54c.59-.24 1.13.57 1.62.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.03-1.58zM12 15.6A3.6 3.6 0 1 1 12 8.4a3.6 3.6 0 0 1 0 7.2z" },
                        { view: "system",   label: qsTr("System"),   icon: "M20 18c1.1 0 1.99-.9 1.99-2L22 5c0-1.1-.9-2-2-2H4c-1.1 0-2 .9-2 2v11c0 1.1.9 2 2 2H0v2h24v-2h-4zM4 5h16v11H4V5z" }
                    ]

                    delegate: Rectangle {
                        required property var modelData

                        Layout.fillWidth: true
                        Layout.preferredHeight: 44
                        color: currentView === modelData.view ? Theme.accentBg : "transparent"

                        // active left border 3px
                        Rectangle {
                            visible: currentView === modelData.view
                            anchors.left: parent.left
                            anchors.top: parent.top
                            anchors.bottom: parent.bottom
                            width: 3
                            color: Theme.accent
                        }

                        RowLayout {
                            anchors.fill: parent
                            anchors.leftMargin: 18
                            spacing: 10

                            // SVG icon (Image loads .svg file; swaps source for active/inactive)
                            Image {
                                Layout.preferredWidth: 18
                                Layout.preferredHeight: 18
                                sourceSize.width: 18
                                sourceSize.height: 18
                                source: currentView === modelData.view
                                        ? "qrc:/qt/qml/ShadowWorker/qml/icons/" + modelData.view + "_active.svg"
                                        : "qrc:/qt/qml/ShadowWorker/qml/icons/" + modelData.view + ".svg"
                            }

                            Text {
                                text: modelData.label
                                color: currentView === modelData.view ? Theme.accent : Theme.muted
                                font.pixelSize: Theme.fontBody
                            }
                        }

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: currentView = modelData.view
                        }
                    }
                }

                Item { Layout.fillHeight: true }
            }
        }

        // ==================== Content ====================
        Rectangle {
            Layout.fillWidth: true
            Layout.fillHeight: true
            color: Theme.bg2

            StackLayout {
                id: contentStack
                anchors.fill: parent
                currentIndex: ["overview", "timeline", "settings", "system"].indexOf(currentView)

                OverviewPage {
                    viewModel: overviewVm
                    onManageAppsRequested: currentView = "settings"
                }
                TimelinePage {
                    viewModel: timelineVm
                }
                SettingsPage {
                    viewModel: settingsVm
                    whitelistViewModel: whitelistVm
                    windowPicker: windowPicker
                }
                SystemPage {}
            }

            // global toast (top-right)
            Toast {
                id: globalToast
                anchors.top: parent.top
                anchors.right: parent.right
                anchors.topMargin: 16
                anchors.rightMargin: 16
            }
        }
    }

    // global toast helper for child pages
    // type: optional, "success" (default) | "error" | "warning"
    function toast(text, type) {
        globalToast.show(text, type)
    }

    // ================================================================
    // Recording window (HTML section 2: recording bubble + result bubble)
    // ================================================================
    // A standalone top-level Window in the same process: frameless,
    // stays-on-top, no taskbar entry. Positioned above the taskbar, centered.
    // Shows even when the main window is minimized to the tray.
    RecordingWindow {
        id: recordingWindow
    }

    // ---- temporary demo trigger (cycle states) ----
    // TODO: replace with a global hotkey once audio capture is wired.
    Rectangle {
        parent: mainWindow.contentItem
        anchors.bottom: mainWindow.contentItem.bottom
        anchors.left: mainWindow.contentItem.left
        anchors.bottomMargin: 24
        anchors.leftMargin: 24
        width: demoTxt.implicitWidth + 24
        height: 32
        radius: 16
        color: demoMa.containsMouse ? Theme.accentBg2 : Theme.bg3
        border.color: demoMa.containsMouse ? Theme.accent : Theme.rule
        border.width: 1
        z: 1000

        Text {
            id: demoTxt
            anchors.centerIn: parent
            text: recordingWindow.state === "hidden"
                  ? qsTr("▶ Demo Record") : qsTr("▶ Next State")
            color: demoMa.containsMouse ? Theme.ink : Theme.muted
            font.pixelSize: 12
        }
        MouseArea {
            id: demoMa
            anchors.fill: parent
            cursorShape: Qt.PointingHandCursor
            hoverEnabled: true
            onClicked: {
                if (recordingWindow.state === "hidden") recordingWindow.show()
                else recordingWindow.advance()
            }
        }
    }

    Component.onCompleted: {
        // pull overview once on startup
        if (overviewVm) overviewVm.refresh()
    }
}
