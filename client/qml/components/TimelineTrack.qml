// TimelineTrack.qml - single-focus horizontal track of category color blocks (v2).
// model: segments [{startTs, endTs, category, state, appName, windowTitle, ...}]
// hourStart/hourEnd define the visible window (default 9..18).
// 三态显示:
//   engaged(强活跃)= 类别色 + 满不透明(opacity 1.0)
//   active (余热)  = 类别色 + 半透明(opacity 0.55)
//   idle   (静默)  = Theme.muted

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var segments: []
    property int hourStart: 9
    property int hourEnd: 18
    // seconds from midnight at track left edge
    readonly property real totalSecs: (hourEnd - hourStart) * 3600

    implicitHeight: 90   // ruler + track + legend

    function secOfDay(ts) {
        var d = new Date(ts * 1000)
        return d.getHours() * 3600 + d.getMinutes() * 60 + d.getSeconds()
    }
    function secToX(sec) {
        var rel = sec - hourStart * 3600
        if (rel < 0) rel = 0
        if (rel > totalSecs) rel = totalSecs
        return (rel / totalSecs) * track.width
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        // ruler
        RowLayout {
            Layout.fillWidth: true
            Layout.preferredHeight: 16
            spacing: 0
            Repeater {
                model: hourEnd - hourStart + 1
                delegate: Item {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    Text {
                        anchors.left: parent.left
                        anchors.verticalCenter: parent.verticalCenter
                        text: (root.hourStart + index) + ":00"
                        color: Theme.muted
                        font.pixelSize: 11
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
                        // clamp segment into visible window
                        property real segStartSec: root.secOfDay(startTs)
                        property real segEndSec: root.secOfDay(endTs)
                        property real x1: Math.max(root.secToX(segStartSec), 0)
                        property real x2: Math.min(root.secToX(segEndSec), track.width)
                        x: x1
                        width: Math.max(x2 - x1, 1)
                        height: parent.height
                        // 三态颜色:idle=muted;engaged=类别色满;active=类别色半透明。
                        color: state === "idle" ? Theme.muted
                             : (category.length > 0 ? Theme.colorOf(category) : Theme.muted)
                        opacity: state === "engaged" ? 1.0
                               : state === "active" ? 0.55
                               : 1.0
                        // no border -> segments flow continuously (no dark gaps between them)

                        MouseArea {
                            anchors.fill: parent
                            hoverEnabled: true
                            onEntered: {
                                var mins = durationMin > 0 ? durationMin : Math.round((segEndSec - segStartSec) / 60)
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
