// OverviewPage.qml - full overview page (v2, M3).
// stats-row 4 cards + tracked apps + heatmap + quick switches + category/app rank + range switch.
// Wires OverviewViewModel; heatmap via GetHeatmap, rank via GetCategoryRank (backend M1).

import QtQuick
import QtQuick.Controls
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

    Flickable {
        anchors.fill: parent
        anchors.margins: 20
        contentWidth: width
        contentHeight: contentCol.implicitHeight
        flickableDirection: Flickable.VerticalFlick
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

        ColumnLayout {
            id: contentCol
            width: parent.width
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
                sub: {
                    if (!viewModel) return qsTr("vs yesterday --")
                    var d = formatDelta(viewModel.minutesDelta, qsTr("min"))
                    return d !== "" ? qsTr("vs yesterday ") + d : qsTr("vs yesterday --")
                }
                subColor: viewModel && viewModel.minutesDelta >= 0 ? Theme.accent : Theme.danger
            }
            StatCard {
                label: qsTr("Interruptions")
                value: viewModel ? viewModel.interruptCount : 0
                sub: {
                    if (!viewModel) return qsTr("vs yesterday --")
                    var d = formatDelta(viewModel.interruptDelta, "")
                    return d !== "" ? qsTr("vs yesterday ") + d : qsTr("vs yesterday --")
                }
                subColor: viewModel && viewModel.interruptDelta <= 0 ? Theme.accent : Theme.danger
            }
            StatCard {
                label: qsTr("Apps Used")
                value: viewModel ? viewModel.appCount : 0
                sub: viewModel && viewModel.activeCategory ? viewModel.activeCategory : "--"
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
                Layout.minimumWidth: 300
                Layout.preferredHeight: 220
                title: qsTr("Activity Heatmap")
                // legend sits in the header, right-aligned with the title
                headerExtra: [
                    Row {
                        spacing: 6
                        Text {
                            text: qsTr("Less")
                            color: Theme.muted
                            font.pixelSize: 12
                            anchors.verticalCenter: parent.verticalCenter
                        }
                        Repeater {
                            model: [0.25, 0.45, 0.65, 0.85, 1.00]
                            delegate: Rectangle {
                                width: 10
                                height: 10
                                radius: 2
                                color: Qt.rgba(0.067, 0.722, 0.506, modelData)
                                anchors.verticalCenter: parent.verticalCenter
                            }
                        }
                        Text {
                            text: qsTr("More")
                            color: Theme.muted
                            font.pixelSize: 12
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                ]

                HeatmapGrid {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 133
                    model: root.fakeHeatmap()  // TEMP fake data; replace with viewModel.heatmap
                }
            }

            // quick switches
            Card {
                Layout.fillWidth: true
                Layout.preferredWidth: 1
                Layout.minimumWidth: 220
                Layout.preferredHeight: 220
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

        // ---- category rank (full width, stacked vertically with app rank) ----
            Card {
                Layout.fillWidth: true
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

        // ---- app rank (full width, below category rank) ----
            Card {
                Layout.fillWidth: true
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

        // (no fillHeight spacer - Flickable handles overflow)
        }
    } // Flickable

    // TEMP: fake heatmap data for visual verification (remove before commit)
    function fakeHeatmap() {
        var out = []
        var today = new Date()
        for (var i = 0; i < 150; i++) {
            var d = new Date(today)
            d.setDate(today.getDate() - i)
            var iso = d.getFullYear() + "-" + ("0"+(d.getMonth()+1)).slice(-2) + "-" + ("0"+d.getDate()).slice(-2)
            var wd = d.getDay()
            var weekend = (wd === 0 || wd === 6)
            if (Math.random() < (weekend ? 0.4 : 0.85)) {
                var mins = weekend ? Math.floor(30 + Math.random()*90) : Math.floor(120 + Math.random()*300)
                var lvl = mins < 60 ? 1 : mins < 180 ? 2 : mins < 300 ? 3 : mins < 420 ? 4 : 5
                out.push({ date: iso, minutes: mins, level: lvl })
            }
        }
        return out
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
