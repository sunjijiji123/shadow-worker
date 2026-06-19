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
    minimumWidth: 1024
    minimumHeight: 600
    title: qsTr("Shadow Worker")
    color: Theme.bg

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
                        { view: "overview", label: qsTr("Overview") },
                        { view: "timeline", label: qsTr("Timeline") },
                        { view: "settings", label: qsTr("Settings") },
                        { view: "system",   label: qsTr("System")   }
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

                            // placeholder icon block (M3 -> real SVG; color follows active state)
                            Rectangle {
                                Layout.preferredWidth: 18
                                Layout.preferredHeight: 18
                                radius: 4
                                color: currentView === modelData.view ? Theme.accent : Theme.muted
                                opacity: currentView === modelData.view ? 1 : 0.4
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
            color: Theme.bg

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
    function toast(text) {
        globalToast.show(text)
    }

    Component.onCompleted: {
        // pull overview once on startup
        if (overviewVm) overviewVm.refresh()
    }
}
