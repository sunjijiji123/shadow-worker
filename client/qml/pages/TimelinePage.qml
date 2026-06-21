// TimelinePage.qml - full timeline page (v2, M3.5).
// date picker + timeline track + worklog/events dual tabs with filters.
// Wires TimelineViewModel (QueryTimeline RPC). Fake data for now.

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var viewModel: null
    property string activeListTab: "worklog"
    property string catFilter: "all"
    property string evFilter: "all"

    // segments/events come from the viewModel (backend). Empty until data is
    // collected; the UI shows an empty-state hint in that case.
    function allSegments() {
        return viewModel && viewModel.segments ? viewModel.segments : []
    }
    function allEvents() {
        return viewModel && viewModel.events ? viewModel.events : []
    }

    function filteredSegments() {
        var segs = allSegments()
        if (catFilter === "all") return segs
        var out = []
        for (var i = 0; i < segs.length; i++) {
            if (segs[i].category === catFilter) out.push(segs[i])
        }
        return out
    }
    function filteredEvents() {
        var evs = allEvents()
        if (evFilter === "all") return evs
        var out = []
        for (var j = 0; j < evs.length; j++) {
            if (evs[j].type === evFilter) out.push(evs[j])
        }
        return out
    }

    // badge bg color for a category (translucent like HTML)
    function catBadgeBg(cat) {
        var c = Theme.colorOf(cat)
        return Qt.rgba(parseFloat(c.r), parseFloat(c.g), parseFloat(c.b), 0.15)
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

            // ---- title bar + date picker + today ----
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

                DatePicker {
                    id: datePicker
                    dateText: viewModel ? viewModel.date : "2026-06-19"
                    onDateSelected: function(d) {
                        if (viewModel) viewModel.date = d
                    }
                }

                // Today button - same height as DatePicker (36)
                Rectangle {
                    width: 64
                    height: 36
                    radius: 8
                    color: todayMa.containsMouse ? Theme.bg2 : Theme.bg3
                    border.color: Theme.rule
                    border.width: 1
                    Text {
                        anchors.centerIn: parent
                        text: qsTr("Today")
                        color: todayMa.containsMouse ? Theme.ink : Theme.muted
                        font.pixelSize: 13
                    }
                    MouseArea {
                        id: todayMa
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            var t = new Date()
                            var iso = t.getFullYear() + "-" + ("0"+(t.getMonth()+1)).slice(-2) + "-" + ("0"+t.getDate()).slice(-2)
                            if (viewModel) viewModel.date = iso
                        }
                    }
                }
            }

            // ---- day view: date + summary (SAME ROW) + track ----
            Card {
                Layout.fillWidth: true

                // date + work summary on one row (HTML: flex gap:12)
                RowLayout {
                    Layout.fillWidth: true
                    spacing: 12

                    Text {
                        text: datePicker.dateText
                        color: Theme.ink
                        font.pixelSize: 14
                        font.weight: Font.DemiBold
                    }
                    Text {
                        // total work minutes (engaged + active, idle 不计) + segment count
                        text: {
                            var segs = root.allSegments()
                            var mins = 0
                            var count = 0
                            for (var i = 0; i < segs.length; i++) {
                                // 用 state 字段判断(修正旧版用 category 误判的 bug):
                                // engaged/active 计入工作时长,idle 不计。
                                var st = segs[i].state
                                if (st === "engaged" || st === "active") {
                                    mins += segs[i].durationMin || 0
                                    count++
                                }
                            }
                            var h = Math.floor(mins / 60)
                            var m = mins % 60
                            return qsTr("Work %1h %2m  ·  %3 active segments")
                                   .arg(h).arg(m).arg(count)
                        }
                        color: Theme.muted
                        font.pixelSize: 12
                    }
                }

                // timeline track
                TimelineTrack {
                    Layout.fillWidth: true
                    segments: filteredSegments()
                }

                // legend
                Row {
                    Layout.topMargin: 8
                    spacing: 14
                    Repeater {
                        model: [
                            {cat: "coding",  label: qsTr("Coding")},
                            {cat: "office",  label: qsTr("Office")},
                            {cat: "browser", label: qsTr("Browser")},
                            {cat: "chat",    label: qsTr("Chat")},
                            {cat: "other",   label: qsTr("Other")},
                            {cat: "idle",    label: qsTr("Idle")}
                        ]
                        delegate: Row {
                            spacing: 4
                            Rectangle { width: 10; height: 10; radius: 2; color: Theme.colorOf(modelData.cat); anchors.verticalCenter: parent.verticalCenter }
                            Text { text: modelData.label; color: Theme.muted; font.pixelSize: 12 }
                        }
                    }
                }
            }

            // ---- list card (worklog / events dual tabs) ----
            Card {
                Layout.fillWidth: true
                Layout.fillHeight: true

                ColumnLayout {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    spacing: 12

                    // tab strip
                    Row {
                        spacing: 8
                        Repeater {
                            model: [
                                {tab: "worklog", label: qsTr("Worklog")},
                                {tab: "events",  label: qsTr("Events")}
                            ]
                            delegate: ColumnLayout {
                                spacing: 4
                                Text {
                                    text: modelData.label
                                    color: activeListTab === modelData.tab ? Theme.accent : Theme.muted
                                    font.pixelSize: 14
                                    MouseArea {
                                        anchors.fill: parent
                                        cursorShape: Qt.PointingHandCursor
                                        onClicked: activeListTab = modelData.tab
                                    }
                                }
                                Rectangle {
                                    width: 50; height: 2
                                    color: activeListTab === modelData.tab ? Theme.accent : "transparent"
                                }
                            }
                        }
                    }

                    // ---- worklog tab ----
                    ColumnLayout {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        visible: activeListTab === "worklog"
                        spacing: 8

                        // category filter chips
                        Row {
                            spacing: 6
                            Repeater {
                                model: [
                                    {v: "all",     label: qsTr("All")},
                                    {v: "coding",  label: qsTr("Coding")},
                                    {v: "browser", label: qsTr("Browser")},
                                    {v: "chat",    label: qsTr("Chat")},
                                    {v: "office",  label: qsTr("Office")}
                                ]
                                delegate: Chip {
                                    text: modelData.label
                                    checked: catFilter === modelData.v
                                    dotColor: modelData.v === "all" ? "transparent" : Theme.colorOf(modelData.v)
                                    onClicked: catFilter = modelData.v
                                }
                            }
                        }

                        ColumnLayout {
                            id: wlCol
                            Layout.fillWidth: true
                            spacing: 0

                                Repeater {
                                    model: root.filteredSegments()
                                    delegate: ColumnLayout {
                                        required property var modelData
                                        Layout.fillWidth: true
                                        spacing: 6
                                        Layout.topMargin: 10
                                        Layout.bottomMargin: 10

                                        // seg-header: [time] [app-icon+name] [cat-badge] [duration]
                                        RowLayout {
                                            Layout.fillWidth: true
                                            spacing: 10

                                            Text {
                                                text: modelData.startTime + " - " + modelData.endTime
                                                color: Theme.ink
                                                font.pixelSize: 13
                                                font.weight: Font.DemiBold
                                            }
                                            // app icon + name (fillWidth to spread across card)
                                            Row {
                                                spacing: 6
                                                Layout.fillWidth: true
                                                Layout.minimumWidth: 100
                                                Rectangle {
                                                    width: 20; height: 20; radius: 5
                                                    color: Theme.colorOf(modelData.category)
                                                    Text {
                                                        anchors.centerIn: parent
                                                        text: modelData.appIcon
                                                        color: "#ffffff"
                                                        font.pixelSize: 9
                                                        font.weight: Font.Bold
                                                    }
                                                }
                                                Text {
                                                    text: modelData.appName
                                                    color: Theme.ink
                                                    font.pixelSize: 13
                                                    anchors.verticalCenter: parent.verticalCenter
                                                }
                                            }
                                            // category badge (colored like HTML)
                                            Rectangle {
                                                width: catBadgeLbl.implicitWidth + 14
                                                height: 18
                                                radius: 4
                                                color: root.catBadgeBg(modelData.category)
                                                Text {
                                                    id: catBadgeLbl
                                                    anchors.centerIn: parent
                                                    text: modelData.category
                                                    color: Theme.colorOf(modelData.category)
                                                    font.pixelSize: 11
                                                }
                                            }
                                            Text {
                                                text: modelData.durationMin + " min"
                                                color: Theme.muted
                                                font.pixelSize: 12
                                            }
                                        }
                                        // summary line (indented, with └ prefix like HTML)
                                        Text {
                                            text: "└ " + modelData.summary
                                            color: Theme.muted
                                            font.pixelSize: 12
                                            Layout.leftMargin: 30
                                            wrapMode: Text.WordWrap
                                            Layout.fillWidth: true
                                        }
                                        Rectangle {
                                            Layout.fillWidth: true
                                            height: 1
                                            color: Theme.rule
                                        }
                                    }
                                }
                            // empty-state hint when there are no segments
                            Text {
                                Layout.fillWidth: true
                                Layout.topMargin: 24
                                visible: root.filteredSegments().length === 0
                                text: qsTr("No activity recorded for this day yet.")
                                color: Theme.muted
                                font.pixelSize: 13
                                horizontalAlignment: Text.AlignHCenter
                            }
                    }

                    // ---- events tab ----
                    ColumnLayout {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        visible: activeListTab === "events"
                        spacing: 8

                        Row {
                            spacing: 6
                            Repeater {
                                model: [
                                    {v: "all",           label: qsTr("All")},
                                    {v: "voice",         label: qsTr("Voice")},
                                    {v: "prompt_inject", label: qsTr("Prompt")},
                                    {v: "screenshot",    label: qsTr("Screenshot")},
                                    {v: "vlm_summary",   label: qsTr("VLM")}
                                ]
                                delegate: Chip {
                                    text: modelData.label
                                    checked: evFilter === modelData.v
                                    dotColor: modelData.v === "all" ? "transparent" : (Theme.eventTypeColor[modelData.v] || "transparent")
                                    onClicked: evFilter = modelData.v
                                }
                            }
                        }

                        ColumnLayout {
                            id: evCol
                            Layout.fillWidth: true
                            spacing: 0

                                Repeater {
                                    model: root.filteredEvents()
                                    delegate: RowLayout {
                                        required property var modelData
                                        Layout.fillWidth: true
                                        spacing: 10
                                        Layout.topMargin: 8
                                        Layout.bottomMargin: 8

                                        Rectangle {
                                            width: 8; height: 8; radius: 4
                                            color: Theme.eventTypeColor[modelData.type] || Theme.muted
                                        }
                                        Text {
                                            text: modelData.time
                                            color: Theme.muted
                                            font.pixelSize: 13
                                        }
                                        Text {
                                            text: modelData.type + ": " + (modelData.text || "")
                                            color: Theme.ink
                                            font.pixelSize: 13
                                            Layout.fillWidth: true
                                            elide: Text.ElideRight
                                        }
                                        Rectangle {
                                            Layout.fillWidth: true
                                            Layout.topMargin: 8
                                            height: 1
                                            color: Theme.rule
                                        }
                                    }
                                }
                            // empty-state hint when there are no events
                            Text {
                                Layout.fillWidth: true
                                Layout.topMargin: 24
                                visible: root.filteredEvents().length === 0
                                text: qsTr("No events recorded for this day yet.")
                                color: Theme.muted
                                font.pixelSize: 13
                                horizontalAlignment: Text.AlignHCenter
                            }
                        }
                    }
                }
            }
        }
    }
    }
}
