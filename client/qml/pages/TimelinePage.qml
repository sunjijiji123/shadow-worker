// TimelinePage.qml - full timeline page (v2, M3.5).
// date picker + timeline track + worklog/events dual tabs with filters.
// Wires TimelineViewModel (QueryTimeline RPC).

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var viewModel: null
    property string activeListTab: "worklog"   // worklog | events
    property string catFilter: "all"
    property string evFilter: "all"

    // filtered segments (respect catFilter)
    function filteredSegments() {
        if (!viewModel || !viewModel.segments) return []
        if (catFilter === "all") return viewModel.segments
        var out = []
        for (var i = 0; i < viewModel.segments.length; i++) {
            var s = viewModel.segments[i]
            if (s.category === catFilter || (catFilter === "idle" && s.state === "idle")) out.push(s)
        }
        return out
    }
    function filteredEvents() {
        if (!viewModel || !viewModel.events) return []
        if (evFilter === "all") return viewModel.events
        var out = []
        for (var j = 0; j < viewModel.events.length; j++) {
            if (viewModel.events[j].type === evFilter) out.push(viewModel.events[j])
        }
        return out
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 20
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
                dateText: viewModel ? viewModel.date : ""
                onDateSelected: function(d) {
                    if (viewModel) viewModel.date = d
                }
            }

            Button {
                text: qsTr("Today")
                small: true
                onClicked: {
                    var t = new Date()
                    var iso = t.getFullYear() + "-" + ("0"+(t.getMonth()+1)).slice(-2) + "-" + ("0"+t.getDate()).slice(-2)
                    if (viewModel) viewModel.date = iso
                }
            }
        }

        // ---- day summary + timeline track ----
        Card {
            Layout.fillWidth: true
            title: datePicker.dateText
            description: qsTr("Work log timeline")

            ColumnLayout {
                Layout.fillWidth: true
                spacing: 12

                Text {
                    text: viewModel && viewModel.segments
                          ? qsTr("%1 segment(s)").arg(viewModel.segments.length)
                          : qsTr("No data")
                    color: Theme.muted
                    font.pixelSize: 12
                }

                TimelineTrack {
                    Layout.fillWidth: true
                    segments: root.filteredSegments()
                }

                // legend
                Row {
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
                                width: parent.implicitWidth
                                height: 2
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
                                onClicked: catFilter = modelData.v
                            }
                        }
                    }

                    ScrollView {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        clip: true

                        ColumnLayout {
                            width: parent ? parent.width : 600
                            spacing: 0

                            Repeater {
                                model: root.filteredSegments()
                                delegate: ColumnLayout {
                                    required property var modelData
                                    Layout.fillWidth: true
                                    spacing: 4

                                    RowLayout {
                                        Layout.fillWidth: true
                                        spacing: 10
                                        Text {
                                            text: modelData.startTime + " - " + modelData.endTime
                                            color: Theme.ink
                                            font.pixelSize: 13
                                            font.weight: Font.DemiBold
                                        }
                                        Text {
                                            text: modelData.appName
                                            color: Theme.ink
                                            font.pixelSize: 13
                                            Layout.fillWidth: true
                                        }
                                        Rectangle {
                                            width: badge.implicitWidth + 14
                                            height: 18
                                            radius: 4
                                            color: Qt.rgba(0.231, 0.510, 0.965, 0.15)
                                            Text {
                                                id: badge
                                                anchors.centerIn: parent
                                                text: modelData.category
                                                color: Theme.colorOf(modelData.category)
                                                font.pixelSize: 11
                                            }
                                        }
                                        Text {
                                            text: modelData.durationMin + "min"
                                            color: Theme.muted
                                            font.pixelSize: 12
                                        }
                                    }
                                    Text {
                                        visible: modelData.windowTitle && modelData.windowTitle.length > 0
                                        text: modelData.windowTitle
                                        color: Theme.muted
                                        font.pixelSize: 12
                                        Layout.leftMargin: 30
                                    }
                                    Rectangle {
                                        Layout.fillWidth: true
                                        Layout.topMargin: 8
                                        height: 1
                                        color: Theme.rule
                                    }
                                }
                            }

                            Text {
                                visible: root.filteredSegments().length === 0
                                text: qsTr("No segments for this day")
                                color: Theme.muted
                                font.pixelSize: 13
                                Layout.alignment: Qt.AlignHCenter
                                Layout.topMargin: 32
                            }
                        }
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
                                onClicked: evFilter = modelData.v
                            }
                        }
                    }

                    ScrollView {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        clip: true

                        ColumnLayout {
                            width: parent ? parent.width : 600
                            spacing: 0

                            Repeater {
                                model: root.filteredEvents()
                                delegate: RowLayout {
                                    required property var modelData
                                    Layout.fillWidth: true
                                    spacing: 10

                                    Rectangle {
                                        width: 10; height: 10; radius: 5
                                        color: Theme.eventTypeColor[modelData.type] || Theme.muted
                                        Layout.alignment: Qt.AlignVCenter
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
                                        Layout.fillHeight: true
                                    }
                                    Rectangle {
                                        Layout.fillWidth: true
                                        Layout.preferredHeight: 1
                                        color: Theme.rule
                                    }
                                }
                            }

                            Text {
                                visible: root.filteredEvents().length === 0
                                text: qsTr("No events for this day")
                                color: Theme.muted
                                font.pixelSize: 13
                                Layout.alignment: Qt.AlignHCenter
                                Layout.topMargin: 32
                            }
                        }
                    }
                }
            }
        }
    }
}
