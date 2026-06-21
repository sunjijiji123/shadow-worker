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

    // badge bg color for a category (translucent like HTML)
    function catBadgeBg(cat) {
        var c = Theme.colorOf(cat)
        return Qt.rgba(parseFloat(c.r), parseFloat(c.g), parseFloat(c.b), 0.15)
    }

    // formatDuration 把秒数格式化为智能进位的时长文本。
    //   < 60s    → "45s"
    //   < 3600s  → "12 min"（秒部分舍去）
    //   ≥ 3600s  → "1h 23min"（整点小时不显示 0min）
    // 供顶部统计渲染 ViewModel.activeDurationSec() 返回的秒数。
    function formatDuration(sec) {
        if (sec < 60) return Math.max(0, sec) + "s"
        if (sec < 3600) return Math.floor(sec / 60) + " min"
        var h = Math.floor(sec / 3600)
        var m = Math.floor((sec % 3600) / 60)
        return m > 0 ? (h + "h " + m + "min") : (h + "h")
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
                            if (viewModel) {
                                viewModel.date = iso
                                // setDate 对同一天有短路保护（不重复 refresh），
                                // Today 通常就是当天，故显式强制刷新一次。
                                viewModel.refresh()
                            }
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
                        // 统计上移到 ViewModel：engaged/active 段的总时长 + 段数。
                        // Model 变化时（replaceAll 内 dataChanged/reset）触发重算。
                        text: viewModel ? qsTr("Work %1  ·  %2 active segments")
                                          .arg(root.formatDuration(viewModel.activeDurationSec()))
                                          .arg(viewModel.activeSegmentCount())
                                      : ""
                        color: Theme.muted
                        font.pixelSize: 12
                    }
                }

                // timeline track
                TimelineTrack {
                    Layout.fillWidth: true
                    segments: viewModel ? viewModel.segments : null
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

                    // loading indicator: 异步 gRPC 查询期间显示，避免空白被误认为卡死
                    Text {
                        Layout.fillWidth: true
                        Layout.topMargin: 24
                        visible: viewModel && viewModel.loading
                        text: qsTr("Loading…")
                        color: Theme.muted
                        font.pixelSize: 13
                        horizontalAlignment: Text.AlignHCenter
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
                                    checked: (viewModel && viewModel.catFilter || "all") === modelData.v
                                    dotColor: modelData.v === "all" ? "transparent" : Theme.colorOf(modelData.v)
                                    onClicked: if (viewModel) viewModel.catFilter = modelData.v
                                }
                            }
                        }

                        ColumnLayout {
                            id: wlCol
                            Layout.fillWidth: true
                            spacing: 0

                                Repeater {
                                    model: viewModel ? viewModel.segments : null
                                    delegate: ColumnLayout {
                                        // 用 required property 声明 roles（Qt6 推荐范式），
                                        // 替代旧的 modelData.xxx。Model 变化时只更新这些绑定。
                                        required property string startTime
                                        required property string endTime
                                        required property string appName
                                        required property string category
                                        required property string state
                                        required property string summary
                                        required property string durationText

                                        Layout.fillWidth: true
                                        spacing: 6
                                        Layout.topMargin: 10
                                        Layout.bottomMargin: 10

                                        // seg-header: [time] [app-icon+name] [cat-badge] [duration]
                                        RowLayout {
                                            Layout.fillWidth: true
                                            spacing: 10

                                            Text {
                                                text: startTime + " - " + endTime
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
                                                    color: Theme.colorOf(category)
                                                    Text {
                                                        anchors.centerIn: parent
                                                        // 后端无 app_icon 字段，取 appName 首字母兜底
                                                        text: appName.substring(0, 1).toUpperCase()
                                                        color: "#ffffff"
                                                        font.pixelSize: 9
                                                        font.weight: Font.Bold
                                                    }
                                                }
                                                Text {
                                                    text: appName
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
                                                color: root.catBadgeBg(category)
                                                Text {
                                                    id: catBadgeLbl
                                                    anchors.centerIn: parent
                                                    text: category
                                                    color: Theme.colorOf(category)
                                                    font.pixelSize: 11
                                                }
                                            }
                                            Text {
                                                text: durationText
                                                color: Theme.muted
                                                font.pixelSize: 12
                                            }
                                        }
                                        // summary line (indented, with └ prefix like HTML)
                                        // 无摘要时不显示该行（VLM 摘要由后端惰性回填，可能为空）
                                        Text {
                                            visible: summary.length > 0
                                            text: "└ " + summary
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
                                visible: !viewModel || viewModel.segments.rowCount === 0
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
                                    checked: (viewModel && viewModel.evFilter || "all") === modelData.v
                                    dotColor: modelData.v === "all" ? "transparent" : (Theme.eventTypeColor[modelData.v] || "transparent")
                                    onClicked: if (viewModel) viewModel.evFilter = modelData.v
                                }
                            }
                        }

                        ColumnLayout {
                            id: evCol
                            Layout.fillWidth: true
                            spacing: 0

                                Repeater {
                                    model: viewModel ? viewModel.events : null
                                    delegate: RowLayout {
                                        required property string time
                                        required property string type
                                        required property string text
                                        Layout.fillWidth: true
                                        spacing: 10
                                        Layout.topMargin: 8
                                        Layout.bottomMargin: 8

                                        Rectangle {
                                            width: 8; height: 8; radius: 4
                                            color: Theme.eventTypeColor[type] || Theme.muted
                                        }
                                        Text {
                                            text: time
                                            color: Theme.muted
                                            font.pixelSize: 13
                                        }
                                        Text {
                                            text: type + ": " + text
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
                                visible: !viewModel || viewModel.events.rowCount === 0
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
