// TimelinePage.qml - full timeline page (v2, M3.5).
// date picker + timeline track + worklog/events dual tabs with filters.
// Wires TimelineViewModel (QueryTimeline RPC).
//
// 布局（重构后，"顶部固定 + 列表内滚"）：
//   ColumnLayout (root, 不滚动)
//   ├── 标题 + DatePicker + Today          [固定]
//   ├── Card: 日期摘要 + TimelineTrack     [固定，画全天轨道]
//   └── Card (fillHeight):
//       ├── tab (Worklog/Events)            [固定]
//       ├── filter chips                    [固定]
//       └── StackLayout: worklog/events ListView  [此区滚动，虚拟化]
//
// worklog/events 改用 ListView：只实例化可视区 delegate，配合
// RoleFilterProxyModel 增量过滤，切 catFilter 不再全量重建。

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

    // eventTypeLabel 把后端 type 映射成显示标签。
    // 翻译由 qsTr 后续统一走 .ts 翻译文件，这里只做 type→可读名的映射，
    // 不硬编码中文（中文等翻译文件统一管理）。
    function eventTypeLabel(type) {
        var m = {
            "voice": qsTr("voice"),
            "prompt_inject": qsTr("prompt"),
            "screenshot": qsTr("screenshot"),
            "vlm_summary": qsTr("vlm")
        }
        return m[type] || type
    }

    // formatDuration 把秒数格式化为智能进位的时长文本。
    function formatDuration(sec) {
        if (sec < 60) return Math.max(0, sec) + "s"
        if (sec < 3600) return Math.floor(sec / 60) + " min"
        var h = Math.floor(sec / 3600)
        var m = Math.floor((sec % 3600) / 60)
        return m > 0 ? (h + "h " + m + "min") : (h + "h")
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

            // timeline track —— 绑 allSegments（全量 source），不随 catFilter 变。
            TimelineTrack {
                Layout.fillWidth: true
                segments: viewModel ? viewModel.allSegments : null
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
        // fillHeight: 撑满剩余高度，内部 ListView 在此区滚动。
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

                    // worklog ListView（虚拟化）—— 切 catFilter 时只增删差异行 + 只重建可视 delegate。
                    // segments 返回 proxy（已按 category 过滤）。
                    ListView {
                        id: worklogList
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        clip: true
                        // ListView.spacing 控制 delegate 之间的间距。
                        // 注意：delegate 根项的 Layout.topMargin/bottomMargin 在 ListView
                        // 里不生效（ListView 用 implicitHeight，忽略 Layout margin），
                        // 故用 ListView.spacing 拉开行距。
                        spacing: 14
                        boundsBehavior: Flickable.StopAtBounds
                        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

                        model: viewModel ? viewModel.segments : null

                        // 空态：proxy 过滤后无行时显示提示（覆盖在列表上方）。
                        // 用 Text 作为 header 会在有数据时也占位，故用独立的 visible Text。
                        Text {
                            anchors.centerIn: parent
                            visible: viewModel && worklogList.count === 0
                            text: qsTr("No activity recorded for this day yet.")
                            color: Theme.muted
                            font.pixelSize: 13
                        }

                        delegate: ColumnLayout {
                            // seg-row: 匹配线框稿 .seg-row（padding:10px 0 + border-bottom）。
                            // 用 required property 声明 roles（Qt6 推荐范式）。
                            //
                            // ListView 高度坑：delegate 根项的 Layout.topMargin/bottomMargin
                            // 在 ListView 里不生效（ListView 用 implicitHeight，忽略 Layout
                            // margin）。行距改用 ListView.spacing 控制（在 worklogList 上设）。
                            // 这里只控制 delegate 内部内容的垂直间距。
                            required property string startTime
                            required property string endTime
                            required property string appName
                            required property string category
                            required property string state
                            required property string summary
                            required property string durationText

                            width: worklogList.width
                            spacing: 10

                            // seg-header: [time] [app-icon+name(flex)] [cat-badge] [duration]
                            // 与线框稿 .seg-header 一致：display:flex; gap:10px。
                            RowLayout {
                                Layout.fillWidth: true
                                Layout.topMargin: 4
                                Layout.bottomMargin: 4
                                spacing: 10

                                Text {
                                    // 线框稿 .seg-time: nowrap + tabular-nums。
                                    // 固定宽度让整列对齐：等宽数字保证字符等宽，
                                    // 固定宽度保证 "21:16 - 21:19" 与 "21:11 - 21:16"
                                    // 占同样像素（连字符/空格位置一致）。
                                    text: startTime + " - " + endTime
                                    color: Theme.ink
                                    font.pixelSize: 13
                                    font.weight: Font.DemiBold
                                    font.features: ["tnum"]
                                    Layout.preferredWidth: 115
                                    Layout.alignment: Qt.AlignVCenter
                                }
                                // seg-app: app icon + name，flex:1 撑开占满中间空间。
                                RowLayout {
                                    spacing: 6
                                    Layout.fillWidth: true
                                    Layout.minimumWidth: 100
                                    Rectangle {
                                        Layout.alignment: Qt.AlignVCenter
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
                                        Layout.fillWidth: true
                                        elide: Text.ElideRight
                                    }
                                }
                                // seg-cat-badge: 类别色块（线框稿 .seg-cat-badge）
                                Rectangle {
                                    Layout.alignment: Qt.AlignVCenter
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
                                // seg-duration: 最右，固定宽度对齐。
                                // 按 "60 min" 量级设定宽度（最长格式是 "1h 23min"），
                                // 所有行 duration 右对齐到同一列，视觉整齐。
                                Text {
                                    text: durationText
                                    color: Theme.muted
                                    font.pixelSize: 12
                                    font.features: ["tnum"]
                                    Layout.preferredWidth: 60
                                    horizontalAlignment: Text.AlignRight
                                    Layout.alignment: Qt.AlignVCenter
                                }
                            }
                            // seg-summary: 缩进 + └ 前缀（线框稿 margin-left:30px, ::before content:'└'）
                            // 无摘要时不显示（VLM 摘要由后端惰性回填，可能为空）
                            Text {
                                visible: summary.length > 0
                                text: "└ " + summary
                                color: Theme.muted
                                font.pixelSize: 12
                                Layout.leftMargin: 30
                                wrapMode: Text.WordWrap
                                Layout.fillWidth: true
                            }
                            // 分割线（线框稿 .seg-row border-bottom）
                            Rectangle {
                                Layout.fillWidth: true
                                height: 1
                                color: Theme.rule
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
                                checked: (viewModel && viewModel.evFilter || "all") === modelData.v
                                dotColor: modelData.v === "all" ? "transparent" : (Theme.eventTypeColor[modelData.v] || "transparent")
                                onClicked: if (viewModel) viewModel.evFilter = modelData.v
                            }
                        }
                    }

                    // events ListView（虚拟化）—— events 返回 proxy（已按 type 过滤）。
                    // spacing 控制行距（同 worklog：delegate 根的 Layout margin 在 ListView 不生效）。
                    ListView {
                        id: eventsList
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        clip: true
                        spacing: 14
                        boundsBehavior: Flickable.StopAtBounds
                        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

                        model: viewModel ? viewModel.events : null

                        Text {
                            anchors.centerIn: parent
                            visible: viewModel && eventsList.count === 0
                            text: qsTr("No events recorded for this day yet.")
                            color: Theme.muted
                            font.pixelSize: 13
                        }

                        delegate: ColumnLayout {
                            // app-row: 匹配线框稿 .app-row（gap:12, padding:10px 0, border-bottom）。
                            // 行距由 eventsList.spacing 控制（ListView 里 delegate 根的
                            // Layout margin 不生效，见 worklogList 注释）。
                            //
                            // 注意：property 名用 evText 而非 text。text 是 QML 内置属性，
                            // required property string text 会与内置机制冲突导致值绑不上
                            //（实测 text role 永远为空）。Model 的 roleNames 也相应用 "evText"。
                            required property string time
                            required property string type
                            required property string evText
                            width: eventsList.width
                            spacing: 10

                            RowLayout {
                                Layout.fillWidth: true
                                Layout.topMargin: 4
                                Layout.bottomMargin: 4
                                spacing: 12

                                // app-color: 10×10 圆点（线框稿 .app-color border-radius:50%）
                                Rectangle {
                                    Layout.alignment: Qt.AlignVCenter
                                    width: 10; height: 10; radius: 5
                                    color: Theme.eventTypeColor[type] || Theme.muted
                                }
                                // app-name: 合并显示 "09:12 语音：帮我把..."（线框稿 .app-name flex:1）
                                // 时间 + 类型 + 文本 全部拼在一行，对齐线框稿设计。
                                Text {
                                    text: time + "  " + root.eventTypeLabel(type) + "：" + evText
                                    color: Theme.ink
                                    font.pixelSize: 14
                                    Layout.fillWidth: true
                                    elide: Text.ElideRight
                                }
                            }
                            // 分割线（线框稿 .app-row border-bottom）
                            Rectangle {
                                Layout.fillWidth: true
                                height: 1
                                color: Theme.rule
                            }
                        }
                    }
                }
            }
        }
    }
}
