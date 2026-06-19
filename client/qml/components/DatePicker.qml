// DatePicker.qml - date navigator with calendar Popup (v2 timeline header).
// Uses QtQuick.Controls Popup so the calendar floats on the Overlay layer
// (above all page content, no z-order issues).
// property dateText: "yyyy-MM-dd". signal dateSelected("yyyy-MM-dd").

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property string dateText: ""
    property var viewMonth: new Date()

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
                        calPopup.open()
                    }
                }
            }

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

    // --- calendar Popup (Overlay layer -> floats above everything) ---
    Popup {
        id: calPopup
        x: (bar.width - width) / 2
        y: bar.height + 8
        width: 260
        height: 300
        padding: 0
        modal: true
        closePolicy: Popup.CloseOnEscape | Popup.CloseOnPressOutside
        background: Rectangle {
            color: Theme.bg3
            border.color: Theme.rule
            border.width: 1
            radius: 12
        }

        onOpened: {
            root.viewMonth = root.dateText ? root.parseDate(root.dateText) : new Date()
        }

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 12
            spacing: 8

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
                        required property var modelData
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
                                        calPopup.close()
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    function buildDays(monthDate) {
        var y = monthDate.getFullYear()
        var m = monthDate.getMonth()
        var first = new Date(y, m, 1)
        var startWd = first.getDay()
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
