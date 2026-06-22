// TimelineTrack.qml - single-focus horizontal track of category color blocks (v2).
// model: segments [{startTs, endTs, category, state, appName, windowTitle, ...}]
//
// 窗口由后端动态计算（windowStartTs / windowEndTs，unix 秒，已整点取整）：
//   floor(首条事件整点) ~ ceil(末条事件整点)，minWindow 2h，今天 end 含 now。
// 刻度强制整点：从 windowStart 到 windowEnd 按窗口跨度选 1h（≤12h）或 2h（>12h）步进。
// 离开断段后，段与段之间的"空档"在轨道上显示为空白（背景色），一眼可见"我离开过多久"。
//
// 三态显示:
//   engaged(强活跃)= 类别色 + 满不透明(opacity 1.0)
//   active (余热)  = 类别色 + 半透明(opacity 0.55)
//   idle   (静默)  = #2A2A2A（思考/读文档，区别于"离开"的空白断档）

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var segments: []
    // 可视窗口边界（unix 秒，后端算好传入）。TimelinePage 绑 viewModel。
    property int windowStartTs: 0
    property int windowEndTs: 0

    readonly property int windowSecs: windowEndTs - windowStartTs

    implicitHeight: 90   // ruler + track + legend

    // 绝对时间 → x 坐标。窗口外 clamp 到边界。
    function tsToX(ts) {
        if (windowSecs <= 0) return 0
        var rel = ts - windowStartTs
        if (rel < 0) rel = 0
        if (rel > windowSecs) rel = windowSecs
        return (rel / windowSecs) * track.width
    }

    // 整点刻度（unix 秒）：从 windowStart 到 windowEnd，按 1h（窗口≤12h）或 2h（>12h）步进。
    // 刻度边界取本地整点（new Date 自动按本地时区），故本地显示落在整点。
    function hourTicks() {
        if (windowSecs <= 0) return []
        var startMs = windowStartTs * 1000
        var endMs = windowEndTs * 1000
        // 起点对齐到本地整点（向上取整到下一个整点，windowStart 已是 UTC 整点，
        // 本地时区若是整时区则 startMs 本身就是本地整点；分数时区才需要此对齐）。
        var d0 = new Date(startMs)
        d0.setMinutes(0, 0, 0)
        var curMs = d0.getTime()
        if (curMs < startMs) curMs += 3600 * 1000
        // 步长：≤12h 用 1h，>12h 用 2h（避免刻度拥挤）。
        var stepMs = (windowSecs <= 12 * 3600) ? 3600 * 1000 : 2 * 3600 * 1000
        var ticks = []
        while (curMs <= endMs) {
            ticks.push(curMs / 1000)
            curMs += stepMs
        }
        return ticks
    }

    // 格式化刻度标签：unix 秒 → "HH:00"（本地时区）。
    function formatTick(ts) {
        var d = new Date(ts * 1000)
        var hh = d.getHours()
        return (hh < 10 ? "0" + hh : hh) + ":00"
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // ruler
        Item {
            Layout.fillWidth: true
            Layout.preferredHeight: 16

            Repeater {
                model: root.hourTicks()
                delegate: Item {
                    // 每个刻度用绝对 x 定位。文本对齐策略避免 "08:00" 溢出卡片：
                    //   最右刻度（x 接近轨道右沿）→ 文本右对齐到刻度，文字向左展开，不溢出。
                    //   最左刻度 → 文本左对齐到刻度（默认）。
                    //   中间刻度 → 文本居中于刻度。
                    // 用 horizontalAlignment 而非切换 anchors（切换 anchors 运行期易冲突）。
                    property real tickX: root.tsToX(modelData)
                    property bool isRightmost: tickX > track.width - 40  // 约 "08:00" 5 字符宽
                    property bool isLeftmost: tickX < 4
                    x: isRightmost ? tickX - 40            // 右对齐：文本框左边界往左挪
                     : isLeftmost ? tickX                  // 左对齐：文本框左边界 = 刻度
                     : tickX - 20                          // 居中：文本框中心 = 刻度
                    width: 40
                    height: 16
                    Text {
                        anchors.fill: parent
                        horizontalAlignment: isRightmost ? Text.AlignRight
                                       : isLeftmost ? Text.AlignLeft
                                       : Text.AlignHCenter
                        verticalAlignment: Text.AlignVCenter
                        text: root.formatTick(modelData)
                        color: Theme.muted
                        font.pixelSize: 11
                        font.features: ["tnum"]
                    }
                }
            }
        }

        // track
        Item {
            id: trackWrap
            Layout.fillWidth: true
            Layout.preferredHeight: 36
            Layout.topMargin: 20

            Rectangle {
                id: track
                anchors.fill: parent
                color: Theme.bg
                radius: 6
                clip: true

                Repeater {
                    model: root.segments
                    delegate: Rectangle {
                        required property int startTs
                        required property int endTs
                        required property string state
                        required property string category
                        required property int durationMin
                        required property string startTime
                        required property string endTime
                        required property string appName
                        // 段的绝对时间 → x（直接用 unix 秒，不再转 secOfDay）。
                        property real x1: Math.max(root.tsToX(startTs), 0)
                        property real x2: Math.min(root.tsToX(endTs), track.width)
                        x: x1
                        width: Math.max(x2 - x1, 1)
                        height: parent.height
                        // 三态颜色:idle=Theme.colorOf("idle")(中灰，与图例一致);
                        // engaged=类别色满不透明;active=类别色半透明。
                        // idle 用统一中灰而非各 app 色弱化：① 与图例一致（避免"指示色不对"）；
                        // ② 中灰在卡片背景上可见，但弱于工作段，主次清晰（工作 > idle > 离开空白）。
                        // 注意：离开检测上线后，真正的"离开"是段间空白断档（不画色块）；
                        // idle 色块专指"人在但暂时没动（思考/读文档）"。
                        color: state === "idle" ? Theme.colorOf("idle")
                             : (category.length > 0 ? Theme.colorOf(category) : Theme.colorOf("idle"))
                        opacity: state === "engaged" ? 1.0
                               : state === "active" ? 0.55
                               : 1.0
                        // no border -> segments flow continuously (no dark gaps between them)

                        MouseArea {
                            anchors.fill: parent
                            hoverEnabled: true
                            onEntered: {
                                var durSec = endTs - startTs
                                var mins = durationMin > 0 ? durationMin : Math.round(durSec / 60)
                                // 三态标签:idle/engaged/active 各显示对应文字。
                                var cat = state === "idle" ? "idle" : (category.length > 0 ? category : "?")
                                var stateTag = state === "engaged" ? "engaged"
                                             : state === "active" ? "active"
                                             : "idle"
                                var txt = startTime + "-" + endTime
                                         + "  " + appName
                                         + "  [" + cat + "/" + stateTag + "]  " + mins + "min"
                                tip.show(txt, mouseX, mouseY)
                            }
                            onExited: tip.hide()
                        }
                    }
                }
            }

            // tooltip
            Rectangle {
                id: tip
                visible: false
                color: Qt.rgba(0/255, 0/255, 0/255, 0.85)
                border.color: Theme.rule
                border.width: 1
                radius: 6
                width: tipText.implicitWidth + 16
                height: tipText.implicitHeight + 10
                z: 100
                property string txt: ""
                function show(t, mx, my) {
                    txt = t
                    tipText.text = t
                    var px = Math.min(mx + 12, trackWrap.width - tip.width)
                    var py = Math.max(my - tip.height - 6, 0)
                    tip.x = px
                    tip.y = py
                    tip.visible = true
                }
                function hide() { tip.visible = false }
                Text {
                    id: tipText
                    anchors.centerIn: parent
                    color: "#ffffff"
                    font.pixelSize: 12
                }
            }
        }
    }
}
