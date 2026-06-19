// OverviewPage.qml - overview page skeleton (v2).
// M2: wire ViewModel, show basic stat cards + collection apps + placeholders.
// M3 TODO: heatmap / category rank / app rank / today-week-month switch
//          (needs VM interruptCount/heatmap/rank; backend M1 ready).

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var viewModel: null

    // "Manage" button on collection-apps card -> jump to settings (handled by main.qml)
    signal manageAppsRequested()

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 20
        spacing: 16

        // ---- title bar + actions ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 8

            Text {
                text: qsTr("Overview")
                color: Theme.ink
                font.pixelSize: Theme.fontTitle
                font.weight: Font.DemiBold
            }
            Item { Layout.fillWidth: true }

            // today/week/month (M3 -> range; M2 display only)
            Row {
                spacing: 6
                Repeater {
                    model: [qsTr("Today"), qsTr("This Week"), qsTr("This Month")]
                    delegate: Chip {
                        text: modelData
                        checked: index === 0
                    }
                }
            }
            Button {
                text: qsTr("Refresh")
                onClicked: if (viewModel) viewModel.refresh()
            }
        }

        // ---- stats-row 4 cards (M2: current VM fields; M3 add interrupt count) ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 16

            StatCard {
                label: qsTr("Work Duration")
                value: viewModel ? formatMinutes(viewModel.todayMinutes) : "--"
                sub: qsTr("M3: delta vs yesterday")
            }
            StatCard {
                label: qsTr("Interruptions")
                value: qsTr("M3")
                sub: qsTr("needs VM interruptCount")
            }
            StatCard {
                label: qsTr("Apps Used")
                value: viewModel ? (viewModel.apps ? viewModel.apps.length : 0) : "0"
                sub: qsTr("M3: category breakdown")
            }
            StatCard {
                label: qsTr("Current App")
                value: viewModel && viewModel.activeApp ? viewModel.activeApp : "--"
                sub: viewModel && viewModel.collectionStatus ? statusText(viewModel.collectionStatus) : ""
            }
        }

        // ---- collection apps card ----
        Card {
            Layout.fillWidth: true
            title: qsTr("Tracked Apps")
            description: qsTr("Apps in the whitelist. M3 adds horizontal cards + add button.")
            headerExtra: [
                Button {
                    text: qsTr("Manage")
                    onClicked: root.manageAppsRequested()
                }
            ]

            Text {
                color: Theme.muted
                font.pixelSize: Theme.fontSmall
                text: viewModel && viewModel.apps && viewModel.apps.length > 0
                      ? qsTr("%1 app(s) tracked").arg(viewModel.apps.length)
                      : qsTr("No whitelisted apps. Click Manage to add.")
                Layout.fillWidth: true
                wrapMode: Text.WordWrap
            }
        }

        // ---- placeholders: M3 heatmap + quick switches + category/app rank ----
        Card {
            Layout.fillWidth: true
            title: qsTr("Activity Heatmap")
            description: qsTr("M3: GitHub-style contribution grid + GetHeatmap RPC")
            PlaceholderLabel {
                text: qsTr("Pending M3 (needs GetHeatmap RPC + DailyMinutes query; backend M1 ready)")
            }
        }

        Card {
            Layout.fillWidth: true
            title: qsTr("Category Rank / App Rank")
            description: qsTr("M3: GetCategoryRank RPC (backend M1 ready)")
            PlaceholderLabel { text: qsTr("Pending M3") }
        }

        Item { Layout.fillHeight: true }
    }

    // 90 -> "1h 30m"
    function formatMinutes(min) {
        if (!min || min <= 0) return "0m"
        var h = Math.floor(min / 60)
        var m = min % 60
        if (h === 0) return m + "m"
        if (m === 0) return h + "h"
        return h + "h " + m + "m"
    }

    function statusText(s) {
        if (s === "running") return qsTr("Collecting")
        if (s === "paused") return qsTr("Paused")
        return s
    }
}
