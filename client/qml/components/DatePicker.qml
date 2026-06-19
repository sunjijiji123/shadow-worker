// DatePicker.qml - date navigator with calendar popup (v2 timeline header).
// [< date >] + "Today" button. Clicking the date opens a month calendar popup.
// property dateText: "yyyy-MM-dd". signal dateSelected("yyyy-MM-dd").

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property string dateText: ""          // "yyyy-MM-dd"
    property bool popupOpen: false
    // internal: the month being viewed in the popup (QDate as JS Date)
    property var viewMonth: new Date()    // first day of viewed month

    signal dateSelected(string date)

    implicitHeight: 36
    implicitWidth: 280

    function fmtDate(d) {
        var y = d.getFullYear()
        var m = ("0" + (d.getMonth() + 1)).slice(-2)
        var day = ("0" + d.getDate()).slice(-2)
        var wd = ["Sun","Mon","Tue","Wed","Thu","Fri","Sat"][d.getDay()]
        return y + "-" + m + "-" + day + " " + wd
    }
    function fmtMonth(d) {
        var m = ["Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"][d.getMonth()]
        return m + " " + d.getFullYear()
    }
    function isoDate(d) {
        var y = d.getFullYear()
        var m = ("0" + (d.getMonth() + 1)).slice(-2)
        var day = ("0" + d.getDate()).slice(-2)
        return y + "-" + m + "-" + day
    }
    function parseDate(s) {
        var p = s.split("-")
        return new Date(parseInt(p[0]), parseInt(p[1]) - 1, parseInt(p[2]))
    }

    // --- main bar ---
    Rectangle {
        id: bar
        anchors.fill: parent
        color: Theme.bg3
        border.color: Theme.rule
        border.width: 1
        radius: 8

        RowLayout {
            anchors.fill: parent
            anchors.leftMargin: 4
            anchors.rightMargin: 4
            spacing: 0

            // prev day
            Rectangle {
                width: 28; height: 28; radius: 6
                color: prevMa.containsMouse ? Theme.bg2 : "transparent"
                Text { anchors.centerIn: parent; text: "<"; color: Theme.muted; font.pixelSize: 14 }
                MouseArea {
                    id: prevMa
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        var d = root.parseDate(root.dateText)
                        d.setDate(d.getDate() - 1)
                        root.dateSelected(root.isoDate(d))
                    }
                }
            }

            // date display (click -> open popup)
            Item {
                Layout.fillWidth: true
                Layout.fillHeight: true
                Text {
                    anchors.centerIn: parent
                    text: root.dateText ? root.fmtDate(root.parseDate(root.dateText)) : "----"
                    color: Theme.ink
                    font.pixelSize: 14
                    font.weight: Font.Medium
                }
                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        root.viewMonth = root.dateText ? root.parseDate(root.dateText) : new Date()
                        root.popupOpen = !root.popupOpen
                    }
                }
            }

            // next day
            Rectangle {
                width: 28; height: 28; radius: 6
                color: nextMa.containsMouse ? Theme.bg2 : "transparent"
                Text { anchors.centerIn: parent; text: ">"; color: Theme.muted; font.pixelSize: 14 }
                MouseArea {
                    id: nextMa
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        var d = root.parseDate(root.dateText)
                        d.setDate(d.getDate() + 1)
                        root.dateSelected(root.isoDate(d))
                    }
                }
            }
        }
    }

    // --- calendar popup ---
    Rectangle {
        id: popup
        visible: root.popupOpen
        anchors.top: bar.bottom
        anchors.topMargin: 8
        anchors.horizontalCenter: bar.horizontalCenter
        width: 260
        height: 300
        color: Theme.bg3
        border.color: Theme.rule
        border.width: 1
        radius: 12
        z: 100

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 12
            spacing: 8

            // header: prev month / title / next month
            RowLayout {
                Layout.fillWidth: true
                Text {
                    text: "<"; color: Theme.muted; font.pixelSize: 14
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            var d = new Date(root.viewMonth)
                            d.setMonth(d.getMonth() - 1)
                            root.viewMonth = d
                        }
                    }
                }
                Text {
                    Layout.fillWidth: true
                    horizontalAlignment: Text.AlignHCenter
                    text: root.fmtMonth(root.viewMonth)
                    color: Theme.ink
                    font.pixelSize: 14
                    font.weight: Font.DemiBold
                }
                Text {
                    text: ">"; color: Theme.muted; font.pixelSize: 14
                    MouseArea {
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            var d = new Date(root.viewMonth)
                            d.setMonth(d.getMonth() + 1)
                            root.viewMonth = d
                        }
                    }
                }
            }

            // weekday header
            RowLayout {
                Layout.fillWidth: true
                spacing: 0
                Repeater {
                    model: ["S","M","T","W","T","F","S"]
                    delegate: Text {
                        Layout.fillWidth: true
                        horizontalAlignment: Text.AlignHCenter
                        text: modelData
                        color: Theme.muted
                        font.pixelSize: 11
                    }
                }
            }

            // day grid
            GridLayout {
                id: dayGrid
                Layout.fillWidth: true
                Layout.fillHeight: true
                columns: 7
                rowSpacing: 4
                columnSpacing: 4

                Repeater {
                    model: root.buildDays(root.viewMonth)
                    delegate: Item {
                        required property var modelData   // {day: int|0, date: "yyyy-MM-dd", isToday, isSel}
                        Layout.fillWidth: true
                        Layout.fillHeight: true

                        Rectangle {
                            anchors.centerIn: parent
                            width: Math.min(parent.width, parent.height) - 2
                            height: width
                            radius: 6
                            color: modelData.isSel ? Theme.accent : "transparent"
                            border.color: (modelData.isToday && !modelData.isSel) ? Theme.accent : "transparent"
                            border.width: 1
                            visible: modelData.day > 0

                            Text {
                                anchors.centerIn: parent
                                text: modelData.day > 0 ? modelData.day : ""
                                color: modelData.isSel ? "#000000" : Theme.ink
                                font.pixelSize: 12
                                font.weight: modelData.isSel ? Font.DemiBold : Font.Normal
                            }
                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onClicked: {
                                    if (modelData.day > 0) {
                                        root.dateSelected(modelData.date)
                                        root.popupOpen = false
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // click outside to close popup
    MouseArea {
        visible: root.popupOpen
        anchors.fill: parent
        z: -1
        onClicked: root.popupOpen = false
    }

    // build 42-cell grid (6 weeks) for a month; Sun = column 0.
    function buildDays(monthDate) {
        var y = monthDate.getFullYear()
        var m = monthDate.getMonth()
        var first = new Date(y, m, 1)
        var startWd = first.getDay()   // 0..6
        var daysInMonth = new Date(y, m + 1, 0).getDate()
        var today = new Date()
        var todayIso = isoDate(today)
        var selIso = root.dateText
        var cells = []
        for (var i = 0; i < 42; i++) {
            var dayNum = i - startWd + 1
            if (dayNum < 1 || dayNum > daysInMonth) {
                cells.push({day: 0, date: "", isToday: false, isSel: false})
            } else {
                var d = new Date(y, m, dayNum)
                var iso = isoDate(d)
                cells.push({
                    day: dayNum,
                    date: iso,
                    isToday: iso === todayIso,
                    isSel: iso === selIso
                })
            }
        }
        return cells
    }
}
