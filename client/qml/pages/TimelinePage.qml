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
    // 暴露重试等待弹窗给 main.qml（onRetryFinished 里关闭它）。
    property alias retryDialog: retryProgressDialog

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

    // failKindFromMeta 解析 failMeta JSON，返回中文失败提示。
    // failMeta 格式 {"kind":"rate_limit|auth_error|...","detail":"..."}。
    // kind 决定段内显示的简短文字（如"未识别画面内容"/"未采集到画面"），
    // detail 进 hover 气泡。空 JSON 或解析失败时兜底通用提示。
    function failKindFromMeta(meta) {
        if (!meta || meta.length === 0) return qsTr("未识别画面内容")
        try {
            var p = JSON.parse(meta)
            var m = {
                "rate_limit": qsTr("未识别画面内容"),
                "auth_error": qsTr("未识别画面内容"),
                "parse_error": qsTr("未识别画面内容"),
                "capture_failed": qsTr("未采集到画面"),
                "request_failed": qsTr("未识别画面内容")
            }
            return m[p.kind] || qsTr("未识别画面内容")
        } catch(e) {
            return qsTr("未识别画面内容")
        }
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
                    // 统计：engaged/active 段的总时长 + 段数。
                    // 用 Q_PROPERTY 绑定（无括号），refresh 后 NOTIFY 自动刷新。
                    // 之前用 Q_INVOKABLE 方法调用（带括号）导致只首次求值、数据到了不更新。
                    text: viewModel ? qsTr("Work %1  ·  %2 active segments")
                                      .arg(root.formatDuration(viewModel.activeDurationSec))
                                      .arg(viewModel.activeSegmentCount)
                                  : ""
                    color: Theme.muted
                    font.pixelSize: 12
                }
            }

            // timeline track —— 绑 allSegments（全量 source），不随 catFilter 变。
            // 窗口边界由后端动态计算（首末事件整点取整 + minWindow 2h + 今天含 now），
            // TimelineTrack 据此画动态整点刻度。
            TimelineTrack {
                Layout.fillWidth: true
                segments: viewModel ? viewModel.allSegments : null
                windowStartTs: viewModel ? viewModel.windowStartTs : 0
                windowEndTs: viewModel ? viewModel.windowEndTs : 0
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
                                {v: "office",  label: qsTr("Office")},
                                {v: "failed",  label: qsTr("Failed")}
                            ]
                            delegate: Chip {
                                text: modelData.label
                                checked: (viewModel && viewModel.catFilter || "all") === modelData.v
                                dotColor: modelData.v === "all" ? "transparent"
                                         : (modelData.v === "failed" ? Theme.muted : Theme.colorOf(modelData.v))
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
                            required property string failMeta
                            // startTs/endTs 用于"重试失败 VLM"——传给 retryVLMFailures。
                            // 用 required property int 绑定 model 的 startTs/endTs role。
                            // （之前 qint64 会报 "not a type" 崩溃，但 int 是 QML 基本类型没问题。）
                            required property int startTs
                            required property int endTs

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
                            // seg-summary: 缩进 + └ 前缀（线框稿 margin-left:30px）。
                            // 三种状态：
                            //   正常（summary 非空）：└ + 多条摘要（每条换行带时间戳）
                            //   失败（failMeta 非空）：└ ○! 未识别画面内容 + 重试按钮
                            //   未采集（都空）：└ 未采集画面（中性，无感叹号无按钮）
                            ColumnLayout {
                                Layout.leftMargin: 30
                                Layout.fillWidth: true
                                spacing: 4

                                // 摘要/失败/未采集 文本。
                                // 正常：后端已含 └ 前缀 + 后续行空格对齐，直接显示。
                                // 失败：感叹号+└+失败提示 放一行（感叹号在 └ 右边、文本左边）。
                                // 未采集：└ 未采集画面。
                                RowLayout {
                                    Layout.fillWidth: true
                                    spacing: 6

                                    // 行首图标：
                                    //   失败（failMeta 非空）：警告图标（Canvas 空心圆+感叹号），替代 └
                                    //   正常/未采集：└ 前缀含在 Text 里
                                    Item {
                                        visible: failMeta.length > 0
                                        Layout.preferredWidth: 16
                                        Layout.preferredHeight: 16
                                        Layout.alignment: Qt.AlignVCenter

                                        Canvas {
                                            anchors.fill: parent
                                            onPaint: {
                                                var ctx = getContext("2d")
                                                ctx.reset()
                                                ctx.strokeStyle = Theme.muted
                                                ctx.fillStyle = Theme.muted
                                                ctx.lineWidth = 1.5
                                                ctx.beginPath()
                                                ctx.arc(8, 8, 6.5, 0, 2 * Math.PI)
                                                ctx.stroke()
                                                ctx.beginPath()
                                                ctx.moveTo(8, 4.5)
                                                ctx.lineTo(8, 9)
                                                ctx.stroke()
                                                ctx.beginPath()
                                                ctx.arc(8, 11, 0.8, 0, 2 * Math.PI)
                                                ctx.fill()
                                            }
                                        }

                                        // hover 仅覆盖警告图标区域。
                                        MouseArea {
                                            anchors.fill: parent
                                            hoverEnabled: true
                                            cursorShape: Qt.WhatsThisCursor
                                            onContainsMouseChanged: {
                                                if (containsMouse && failMeta.length > 0) {
                                                    var detail = ""
                                                    try { detail = JSON.parse(failMeta).detail || "" } catch(e) {}
                                                    failDetailTip.tipText = detail.length > 0
                                                        ? (root.failKindFromMeta(failMeta) + "\n" + detail)
                                                        : root.failKindFromMeta(failMeta)
                                                    var pos = mapToItem(root, width/2, height)
                                                    failDetailTip.showAt(pos.x, pos.y)
                                                } else {
                                                    failDetailTip.hide()
                                                }
                                            }
                                        }
                                    }

                                    // 文本区域：
                                    //   失败（failMeta 非空）：失败提示 + 重试按钮
                                    //   正常（summary 非空）：多条摘要用 Repeater 逐行渲染（每行独立 Text，精确对齐）
                                    //   未采集：└ 冷却间隔内未采集
                                    // summary 可能是 JSON 数组 [{"time":"09:00","text":"..."}] 或旧格式纯文本。
                                    ColumnLayout {
                                        Layout.fillWidth: true
                                        spacing: 2

                                        // 失败提示（单行）
                                        Text {
                                            visible: failMeta.length > 0
                                            text: root.failKindFromMeta(failMeta)
                                            color: Theme.muted
                                            font.pixelSize: 12
                                            Layout.fillWidth: true
                                        }

                                        // 正常摘要：解析 summary JSON 数组，逐行渲染。
                                        // 每行格式 "└ HH:mm 摘要内容"，统一 leftMargin 无需空格对齐。
                                        Repeater {
                                            // 解析 summary：JSON 数组 → 列表；纯文本 → 单条。
                                            model: {
                                                if (failMeta.length > 0 || summary.length === 0) return []
                                                try {
                                                    var parsed = JSON.parse(summary)
                                                    if (Array.isArray(parsed)) {
                                                        return parsed.map(function(e) {
                                                            return "└ " + e.time + " " + e.text
                                                        })
                                                    }
                                                } catch(e) {}
                                                // 旧格式纯文本：按行分割。
                                                return summary.split("\n").filter(function(l) { return l.length > 0 })
                                            }
                                            delegate: Text {
                                                text: modelData
                                                color: Theme.muted
                                                font.pixelSize: 12
                                                Layout.fillWidth: true
                                                wrapMode: Text.WordWrap
                                            }
                                        }

                                        // 未采集（summary 空 + failMeta 空）
                                        Text {
                                            visible: failMeta.length === 0 && summary.length === 0
                                            text: "└ " + qsTr("冷却间隔内未采集")
                                            color: Theme.muted
                                            font.pixelSize: 12
                                            Layout.fillWidth: true
                                        }
                                    }

                                    // 失败行的「重试」按钮：文本右侧。
                                    // 重试中显示"识别中..."并禁用（viewModel.retrying）。
                                    Rectangle {
                                        visible: failMeta.length > 0
                                        Layout.alignment: Qt.AlignVCenter
                                        width: retryBtn.implicitWidth + 16
                                        height: 22
                                        radius: 4
                                        color: retryMa.containsMouse && !retryMa.disabled ? Theme.accentBg2 : "transparent"
                                        border.width: 1
                                        border.color: Theme.rule
                                        opacity: viewModel && viewModel.retrying ? 0.5 : 1.0

                                        Text {
                                            id: retryBtn
                                            anchors.centerIn: parent
                                            text: (viewModel && viewModel.retrying)
                                                  ? qsTr("识别中...")
                                                  : qsTr("重试")
                                            color: Theme.muted
                                            font.pixelSize: 11
                                        }
                                        MouseArea {
                                            id: retryMa
                                            property bool disabled: viewModel && viewModel.retrying
                                            anchors.fill: parent
                                            hoverEnabled: true
                                            cursorShape: disabled ? Qt.WaitCursor : Qt.PointingHandCursor
                                            enabled: !disabled
                                            onClicked: {
                                                retryConfirm.startTs = startTs
                                                retryConfirm.endTs = endTs
                                                retryConfirm.open()
                                            }
                                        }
                                    }
                                }
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

    // 自定义错误详情气泡（替代系统 ToolTip，样式对齐 Theme）。
    // hover 感叹号时调 showAt(x,y) 定位显示，x/y 是 root 坐标系。
    Item {
        id: failDetailTip
        property string tipText: ""
        property real tipX: 0
        property real tipY: 0
        visible: false
        z: 1000
        width: 300
        height: failTipCol.implicitHeight + 20

        function showAt(mx, my) {
            tipX = mx
            tipY = my
            visible = true
        }
        function hide() { visible = false }

        // 定位在指定坐标下方；靠右边界时翻到左侧。
        x: tipX + 300 + 20 > root.width ? tipX - 300 - 12 : tipX - 8
        y: tipY + 8

        Rectangle {
            anchors.fill: parent
            color: Theme.bg3
            border.width: 1
            border.color: Theme.rule
            radius: 6

            ColumnLayout {
                id: failTipCol
                anchors.fill: parent
                anchors.margins: 10
                spacing: 2

                Text {
                    Layout.fillWidth: true
                    text: failDetailTip.tipText
                    color: Theme.muted
                    font.pixelSize: 11
                    wrapMode: Text.WordWrap
                }
            }
        }
        // 点击关闭。
        MouseArea {
            anchors.fill: parent
            onClicked: failDetailTip.hide()
        }
    }

    // 重试确认弹窗：点击段内「重试」按钮时弹出，二次确认避免误触。
    ConfirmDialog {
        id: retryConfirm
        parent: Overlay.overlay
        property int startTs: 0
        property int endTs: 0
        heading: qsTr("重新识别")
        message: qsTr("将重新识别该截图，可能需要等待几秒。是否继续？")
        confirmText: qsTr("重试")
        onConfirmed: {
            if (viewModel) {
                viewModel.retryVLMFailures(retryConfirm.startTs, retryConfirm.endTs, "")
                retryProgressDialog.open()
            }
        }
    }

    // 重试等待弹窗（模态转圈+超时保护）。
    // retryFinished 信号在 main.qml 全局监听（坑 #15），关闭此弹窗 + toast 结果。
    RetryProgressDialog {
        id: retryProgressDialog
        parent: Overlay.overlay
        onTimeout: {
            var win = ApplicationWindow.window
            if (win && win.toast) win.toast(qsTr("识别超时，请稍后刷新查看结果"), "warning")
        }
    }
}
