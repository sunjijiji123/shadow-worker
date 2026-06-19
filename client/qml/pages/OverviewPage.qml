// OverviewPage.qml - full overview page (v2, M3).
// stats-row 4 cards + tracked apps + heatmap + quick switches + category/app rank + range switch.
// Wires OverviewViewModel; heatmap via GetHeatmap, rank via GetCategoryRank (backend M1).

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var viewModel: null

    signal manageAppsRequested()

    // ---- range chips -> bound to viewModel.range ----
    property var ranges: [
        { val: "day",   label: qsTr("Today") },
        { val: "week",  label: qsTr("This Week") },
        { val: "month", label: qsTr("This Month") }
    ]

    Component.onCompleted: {
        if (viewModel) {
            viewModel.refreshHeatmap(3)
            viewModel.refreshCategoryRank()
        }
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 20
        spacing: 16

        // ---- title bar + range chips + refresh ----
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

            Row {
                spacing: 6
                Repeater {
                    model: root.ranges
                    delegate: Chip {
                        text: modelData.label
                        checked: viewModel && viewModel.range === modelData.val
                        onClicked: if (viewModel) viewModel.range = modelData.val
                    }
                }
            }
            Button {
                text: qsTr("Refresh")
                onClicked: {
                    if (viewModel) {
                        viewModel.refresh()
                        viewModel.refreshHeatmap(3)
                        viewModel.refreshCategoryRank()
                    }
                }
            }
        }

        // ---- stats-row 4 cards ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 16

            StatCard {
                label: viewModel && viewModel.range === "day" ? qsTr("Today Duration")
                      : viewModel && viewModel.range === "week" ? qsTr("Week Duration")
                      : qsTr("Month Duration")
                value: viewModel ? formatMinutes(viewModel.todayMinutes) : "--"
                sub: viewModel ? formatDelta(viewModel.minutesDelta, qsTr("min")) : ""
                subColor: viewModel && viewModel.minutesDelta >= 0 ? Theme.accent : Theme.danger
            }
            StatCard {
                label: qsTr("Interruptions")
                value: viewModel ? viewModel.interruptCount : 0
                sub: viewModel ? formatDelta(viewModel.interruptDelta, "") : ""
                subColor: viewModel && viewModel.interruptDelta <= 0 ? Theme.accent : Theme.danger
            }
            StatCard {
                label: qsTr("Apps Used")
                value: viewModel ? viewModel.appCount : 0
                sub: viewModel && viewModel.activeCategory ? viewModel.activeCategory : ""
            }
            StatCard {
                label: qsTr("Current App")
                value: viewModel && viewModel.activeApp ? viewModel.activeApp : "--"
                sub: viewModel && viewModel.collectionStatus ? statusText(viewModel.collectionStatus) : ""
            }
        }

        // ---- tracked apps (horizontal scroll cards) ----
        Card {
            Layout.fillWidth: true
            title: qsTr("Tracked Apps")
            headerExtra: [
                Button {
                    text: qsTr("Manage")
                    onClicked: root.manageAppsRequested()
                }
            ]

            Flickable {
                Layout.fillWidth: true
                Layout.preferredHeight: 80
                contentWidth: appsRow.implicitWidth
                flickableDirection: Flickable.HorizontalFlick
                clip: true
                boundsBehavior: Flickable.StopAtBounds

                Row {
                    id: appsRow
                    spacing: 16

                    Repeater {
                        model: viewModel && viewModel.apps ? viewModel.apps : []
                        delegate: ColumnLayout {
                            required property var modelData
                            spacing: 4

                            Rectangle {
                                Layout.alignment: Qt.AlignHCenter
                                width: 40
                                height: 40
                                radius: 8
                                color: Theme.colorOf(modelData.category)
                                Text {
                                    anchors.centerIn: parent
                                    text: initials(modelData.name)
                                    color: "#ffffff"
                                    font.pixelSize: 14
                                    font.weight: Font.Bold
                                }
                            }
                            Text {
                                Layout.alignment: Qt.AlignHCenter
                                text: modelData.name
                                color: Theme.ink
                                font.pixelSize: 12
                            }
                            Text {
                                Layout.alignment: Qt.AlignHCenter
                                text: modelData.category
                                color: Theme.muted
                                font.pixelSize: 12
                            }
                        }
                    }

                    // empty hint
                    Text {
                        visible: !(viewModel && viewModel.apps && viewModel.apps.length > 0)
                        text: qsTr("No tracked apps. Click Manage to add.")
                        color: Theme.muted
                        font.pixelSize: Theme.fontSmall
                        Layout.alignment: Qt.AlignVCenter
                    }
                }
            }
        }

        // ---- heatmap + quick switches (two columns) ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 16

            Card {
                Layout.fillWidth: true
                Layout.preferredWidth: 2
                title: qsTr("Activity Heatmap")

                RowLayout {
                    Layout.fillWidth: true
                    spacing: 8

                    // legend
                    Text {
                        text: qsTr("Less")
                        color: Theme.muted
                        font.pixelSize: 12
                    }
                    Repeater {
                        model: [0,1,2,3,4,5]
                        delegate: Rectangle {
                            width: 10
                            height: 10
                            radius: 2
                            color: Qt.rgba(0.067, 0.722, 0.506, [0,0.45,0.60,0.75,0.90,1.00][index])
                        }
                    }
                    Text {
                        text: qsTr("More")
                        color: Theme.muted
                        font.pixelSize: 12
                    }
                    Item { Layout.fillWidth: true }
                }

                HeatmapGrid {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 120
                    model: viewModel && viewModel.heatmap ? viewModel.heatmap : []
                }
            }

            // quick switches
            Card {
                Layout.preferredWidth: 1
                title: qsTr("Quick Switches")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 0

                    QuickSwitchRow {
                        Layout.fillWidth: true
                        title: qsTr("Collection")
                        desc: viewModel && viewModel.collectionStatus === "running"
                              ? qsTr("Currently collecting") : qsTr("Paused")
                        checked: viewModel && viewModel.collectionStatus === "running"
                        onToggled: {
                            if (!viewModel) return
                            if (checked) viewModel.resumeCollection()
                            else viewModel.pauseCollection()
                            toast(qsTr("Saved"))
                        }
                    }
                    QuickSwitchRow {
                        Layout.fillWidth: true
                        title: qsTr("Launch at Startup")
                        desc: qsTr("Auto-start after login")
                        checked: false
                        onToggled: toast(qsTr("Saved"))
                    }
                    QuickSwitchRow {
                        Layout.fillWidth: true
                        title: qsTr("Collect on Start")
                        desc: qsTr("Start collecting immediately on launch")
                        checked: false
                        onToggled: toast(qsTr("Saved"))
                    }
                }
            }
        }

        // ---- category rank + app rank ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 16

            Card {
                Layout.fillWidth: true
                Layout.preferredWidth: 1
                title: qsTr("Category Rank")
                description: viewModel && viewModel.range === "day" ? qsTr("Today category share")
                           : viewModel && viewModel.range === "week" ? qsTr("Week category share")
                           : qsTr("Month category share")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 14

                    Repeater {
                        model: viewModel && viewModel.categoryRank ? viewModel.categoryRank : []
                        delegate: RankBar {
                            Layout.fillWidth: true
                            label: categoryName(modelData.category)
                            value: modelData.percent + "%  " + formatMinutes(modelData.minutes)
                            barRatio: modelData.percent / 100
                            barColor: modelData.color
                            dotColor: modelData.color
                        }
                    }

                    Text {
                        visible: !(viewModel && viewModel.categoryRank && viewModel.categoryRank.length > 0)
                        text: qsTr("No activity yet")
                        color: Theme.muted
                        font.pixelSize: Theme.fontSmall
                    }
                }
            }

            Card {
                Layout.fillWidth: true
                Layout.preferredWidth: 1
                title: qsTr("App Rank")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 14

                    Repeater {
                        model: viewModel && viewModel.apps ? computeAppRank(viewModel.apps) : []
                        delegate: RankBar {
                            Layout.fillWidth: true
                            label: modelData.name
                            value: formatMinutes(modelData.todayMinutes)
                            barRatio: modelData.ratio
                            barColor: Theme.colorOf(modelData.category)
                        }
                    }

                    Text {
                        visible: !(viewModel && viewModel.apps && viewModel.apps.length > 0)
                        text: qsTr("No tracked apps")
                        color: Theme.muted
                        font.pixelSize: Theme.fontSmall
                    }
                }
            }
        }

        Item { Layout.fillHeight: true }
    }

    // ---- helpers ----
    // toast wrapper: walk up to find ApplicationWindow.toast (defined in main.qml)
    function toast(text) {
        var w = root
        while (w && !w.toast) { w = w.parent }
        if (w && w.toast) w.toast(text)
    }
    function formatMinutes(min) {
        if (!min || min <= 0) return "0m"
        var h = Math.floor(min / 60)
        var m = min % 60
        if (h === 0) return m + "m"
        if (m === 0) return h + "h"
        return h + "h " + m + "m"
    }
    function formatDelta(d, unit) {
        if (d === 0 || isNaN(d)) return ""
        var sign = d > 0 ? "+" : ""
        // for interruptions: negative is good (fewer), show as-is
        return sign + d + (unit ? " " + unit : "")
    }
    function statusText(s) {
        if (s === "running") return qsTr("Collecting")
        if (s === "paused") return qsTr("Paused")
        return s
    }
    function initials(name) {
        if (!name) return "?"
        return name.substring(0, 2)
    }
    function categoryName(cat) {
        // English keys; Chinese via i18n later. Map to display label.
        var map = {
            "coding": qsTr("Coding"),
            "office": qsTr("Office"),
            "browser": qsTr("Browser"),
            "chat": qsTr("Chat"),
            "other": qsTr("Other")
        }
        return map[cat] || cat
    }
    // compute app rank with ratio relative to max
    function computeAppRank(apps) {
        if (!apps || apps.length === 0) return []
        var max = 0
        for (var i = 0; i < apps.length; i++) {
            if (apps[i].todayMinutes > max) max = apps[i].todayMinutes
        }
        var out = []
        for (var j = 0; j < apps.length; j++) {
            out.push({
                name: apps[j].name,
                category: apps[j].category,
                todayMinutes: apps[j].todayMinutes,
                ratio: max > 0 ? apps[j].todayMinutes / max : 0
            })
        }
        return out
    }
}
