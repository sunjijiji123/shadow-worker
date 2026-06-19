// HeatmapGrid.qml - GitHub-style contribution grid for the overview page.
// Pure QML (Repeater + Rectangle). Days laid out in 7-row columns (one column per ISO week).
// model: [{ date: "YYYY-MM-DD", minutes: int, level: 0..5 }, ...]
// Cells colored by Theme.accent at level/5 opacity; empty days (level 0) use bg.

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var model: []          // list of {date, minutes, level}
    property string selectedDate: ""

    signal dateClicked(string date)

    // cell geometry
    readonly property int cellSize: 13
    readonly property int cellGap: 4

    implicitHeight: 7 * (cellSize + cellGap) + 16   // 7 rows + month label space
    implicitWidth: 400

    // level -> color
    function levelColor(level) {
        var alpha
        switch (level) {
            case 1: alpha = 0.45; break
            case 2: alpha = 0.60; break
            case 3: alpha = 0.75; break
            case 4: alpha = 0.90; break
            case 5: alpha = 1.00; break
            default: alpha = 0  // level 0 = no data
        }
        return Qt.rgba(0.067, 0.722, 0.506, alpha)   // #10B981decomposed = 16/255,184/255,129/255
    }

    // tooltip (created on demand, single instance)
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
        property string text: ""
        Text {
            id: tipText
            anchors.centerIn: parent
            text: tip.text
            color: "#ffffff"
            font.pixelSize: 12
        }
        function show(txt, mx, my) {
            text = txt
            // position relative to root; clamp inside
            var px = Math.min(Math.max(mx, 0), root.width - tip.width)
            var py = Math.max(my - tip.height - 6, 0)
            tip.x = px
            tip.y = py
            tip.visible = true
        }
        function hide() { tip.visible = false }
    }

    // Grid: one column per week (7 days). We assume model is ordered ascending by date.
    // Compute columns by grouping consecutive days; pad week start so Sunday=column row 0.
    Flickable {
        anchors.fill: parent
        contentWidth: weekRow.implicitWidth
        contentHeight: parent.height
        flickableDirection: Flickable.HorizontalFlick
        clip: true
        boundsBehavior: Flickable.StopAtBounds

        Row {
            id: weekRow
            spacing: root.cellGap
            padding: 0

            Repeater {
                // group model into weeks: list of columns, each column = 7 day slots
                model: buildWeeks(root.model)

                delegate: Column {
                    required property var modelData   // {days: [7 or fewer slots], label}
                    spacing: root.cellGap

                    Text {
                        // month label only on the first column of a month
                        visible: modelData.label !== ""
                        text: modelData.label
                        color: Theme.muted
                        font.pixelSize: 11
                        height: visible ? 14 : 0
                    }
                    Item { height: modelData.label !== "" ? 0 : 14; width: 1 }  // spacer when no label

                    Repeater {
                        model: modelData.days
                        delegate: Rectangle {
                            required property var modelData   // {date, minutes, level} or null
                            width: root.cellSize
                            height: root.cellSize
                            radius: 3
                            color: modelData ? root.levelColor(modelData.level) : Theme.bg
                            border.color: root.selectedDate === (modelData ? modelData.date : "")
                                          ? Theme.accent : "transparent"
                            border.width: root.selectedDate === (modelData ? modelData.date : "") ? 1 : 0

                            MouseArea {
                                anchors.fill: parent
                                hoverEnabled: true
                                onEntered: {
                                    if (modelData) {
                                        var mins = modelData.minutes
                                        var txt = modelData.date + "  " + (mins > 0 ? mins + " min" : "no activity")
                                        var p = parent.mapToItem(root, mouse.x, mouse.y)
                                        tip.show(txt, p.x, p.y)
                                    }
                                }
                                onExited: tip.hide()
                                onClicked: {
                                    if (modelData) root.dateClicked(modelData.date)
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // build weeks from a flat list of {date, minutes, level}.
    // returns [{days: [7 slots], label: "Mon"}]; each slot is the day obj or null.
    function buildWeeks(days) {
        var cols = []
        var currentCol = null
        var currentMonth = -1
        var i = 0
        // map date -> {y,m,d,weekday}
        function parse(s) {
            var parts = s.split("-")
            return { y: parseInt(parts[0]), m: parseInt(parts[1]), d: parseInt(parts[2]) }
        }
        // JS Date: weekday 0=Sun..6=Sat; we want Sun at row 0
        for (var idx = 0; idx < days.length; idx++) {
            var day = days[idx]
            var p = parse(day.date)
            var dt = new Date(Date.UTC(p.y, p.m - 1, p.d))
            var weekday = dt.getUTCDay()   // 0..6
            // start a new column when weekday==0 or no current column
            if (weekday === 0 || currentCol === null) {
                if (currentCol !== null) cols.push(currentCol)
                currentCol = { days: [null,null,null,null,null,null,null], label: "" }
                // month label when month changes
                if (p.m !== currentMonth) {
                    currentMonth = p.m
                    var monthNames = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"]
                    currentCol.label = monthNames[p.m - 1]
                }
            }
            currentCol.days[weekday] = day
        }
        if (currentCol !== null) cols.push(currentCol)
        return cols
    }
}
