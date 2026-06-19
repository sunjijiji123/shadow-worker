// HeatmapGrid.qml - GitHub-style contribution grid, grouped by month (v2 design).
// ALWAYS renders the last `monthsBack` months of dates (like HTML cols=26 weeks).
// Days with no data -> level 0 (dark fill); days in model -> their level.
// model: [{ date: "YYYY-MM-DD", minutes: int, level: 0..5 }, ...]

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var model: []
    property string selectedDate: ""
    property int monthsBack: 5    // months shown; scroll for more

    signal dateClicked(string date)

    readonly property int cellSize: 13
    readonly property int cellGap: 4
    readonly property int monthGap: 26
    // height matches HTML: month label (14) + gap (4) + 7 rows of cells
    // each row = cellSize(13) + cellGap(4), 7 rows -> 7*13 + 6*4 = 115
    readonly property int gridHeight: 14 + 4 + (7 * cellSize + 6 * cellGap)  // = 133
    implicitHeight: gridHeight

    function levelColor(level) {
        var a
        switch (level) {
            case 1: a = 0.45; break
            case 2: a = 0.60; break
            case 3: a = 0.75; break
            case 4: a = 0.90; break
            case 5: a = 1.00; break
            default: a = 0
        }
        return Qt.rgba(0.067, 0.722, 0.506, a)   // #10B981
    }

    function monthLabel(m) {
        var names = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"]
        return names[m - 1]
    }

    Rectangle {
        id: tip
        visible: false
        color: Qt.rgba(0, 0, 0, 0.85)
        border.color: Theme.rule
        border.width: 1
        radius: 6
        width: tipText.implicitWidth + 16
        height: tipText.implicitHeight + 10
        z: 100
        Text {
            id: tipText
            anchors.centerIn: parent
            color: "#ffffff"
            font.pixelSize: 12
        }
        function show(txt, mx, my) {
            tipText.text = txt
            var px = Math.min(Math.max(mx, 0), root.width - tip.width)
            var py = Math.max(my - tip.height - 6, 0)
            tip.x = px
            tip.y = py
            tip.visible = true
        }
        function hide() { tip.visible = false }
    }

    Flickable {
        anchors.fill: parent
        contentWidth: monthRow.implicitWidth
        contentHeight: monthRow.implicitHeight
        flickableDirection: Flickable.HorizontalFlick
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentWidth > width

        Row {
            id: monthRow
            spacing: root.monthGap
            // center horizontally when content fits; Flickable scrolls when it doesn't
            x: Math.max(0, (root.width - implicitWidth) / 2)

            Repeater {
                // always render the last N months, regardless of model
                model: root.buildMonths(root.model, root.monthsBack)

                delegate: ColumnLayout {
                    id: monthCol
                    required property var modelData
                    spacing: root.cellGap

                    Text {
                        text: modelData.label
                        color: Theme.muted
                        font.pixelSize: 11
                        Layout.preferredHeight: 14
                        Layout.leftMargin: 1
                    }

                    Row {
                        spacing: root.cellGap

                        Repeater {
                            model: monthCol.modelData.weekColumns

                            delegate: Column {
                                required property var modelData
                                spacing: root.cellGap

                                Repeater {
                                    model: modelData

                                    delegate: Rectangle {
                                        required property var modelData
                                        width: root.cellSize
                                        height: root.cellSize
                                        radius: 3
                                        // level 0 (no data) -> dark fill (#2a2a2a-ish); else accent by level
                                        color: (modelData && modelData.level > 0)
                                               ? root.levelColor(modelData.level)
                                               : Qt.rgba(0.16, 0.16, 0.17, 1.0)   // #2a2a2a dark empty cell
                                        border.color: "transparent"
                                        border.width: 0

                                        MouseArea {
                                            anchors.fill: parent
                                            hoverEnabled: true
                                            onEntered: {
                                                if (modelData) {
                                                    var mins = modelData.minutes || 0
                                                    var txt = modelData.date + "  " + (mins > 0 ? mins + " min" : "no activity")
                                                    var p = parent.mapToItem(root, parent.width / 2, 0)
                                                    tip.show(txt, p.x, p.y)
                                                }
                                            }
                                            onExited: tip.hide()
                                            onClicked: if (modelData) root.dateClicked(modelData.date)
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // Build the last `monthsBack` months of cells. Every day in range becomes a cell
    // (level from model lookup, or 0). Cells flow column-by-column within each month
    // (7 rows; first day aligned to its weekday via leading nulls).
    function buildMonths(model, monthsBack) {
        // index model by date for O(1) lookup
        var byDate = {}
        if (model) {
            for (var i = 0; i < model.length; i++) {
                byDate[model[i].date] = model[i]
            }
        }
        function iso(d) {
            var y = d.getFullYear()
            var m = ("0" + (d.getMonth() + 1)).slice(-2)
            var day = ("0" + d.getDate()).slice(-2)
            return y + "-" + m + "-" + day
        }
        // determine start: first day of the month that is `monthsBack` months before today
        var today = new Date()
        var startY = today.getFullYear()
        var startM = today.getMonth() - (monthsBack - 1)
        while (startM < 0) { startM += 12; startY -= 1 }

        var out = []
        for (var mi = 0; mi < monthsBack; mi++) {
            var y = startY
            var m = startM + mi
            while (m > 11) { m -= 12; y += 1 }
            var label = root.monthLabel(m + 1)
            var daysInMonth = new Date(y, m + 1, 0).getDate()
            // build flat slot list: leading nulls for weekday of day 1, then each day
            var firstWd = new Date(y, m, 1).getDay()   // 0..6
            var slots = []
            for (var p = 0; p < firstWd; p++) slots.push(null)
            for (var d = 1; d <= daysInMonth; d++) {
                var dd = new Date(y, m, d)
                var isoStr = iso(dd)
                var entry = byDate[isoStr]
                if (entry) {
                    slots.push(entry)
                } else {
                    // future days (after today) -> null (don't render)
                    if (dd > today) {
                        slots.push(null)
                    } else {
                        slots.push({ date: isoStr, minutes: 0, level: 0 })
                    }
                }
            }
            // pad to multiple of 7
            while (slots.length % 7 !== 0) slots.push(null)
            // chunk into columns of 7
            var cols = []
            for (var c = 0; c < slots.length; c += 7) {
                cols.push([slots[c], slots[c+1], slots[c+2], slots[c+3], slots[c+4], slots[c+5], slots[c+6]])
            }
            out.push({ label: label, weekColumns: cols })
        }
        return out
    }
}
