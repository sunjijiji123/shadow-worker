// TimelineTrack.qml - single-focus horizontal track of category color blocks (v2).
// model: segments [{startTs, endTs, category, state, appName, windowTitle, ...}]
// hourStart/hourEnd define the visible window (default 9..18).
// idle segments use Theme.muted; others use Theme.colorOf(category).

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
                        required property var modelData
                        // clamp segment into visible window
                        property real segStartSec: root.secOfDay(modelData.startTs)
                        property real segEndSec: root.secOfDay(modelData.endTs)
                        property real x1: Math.max(root.secToX(segStartSec), 0)
                        property real x2: Math.min(root.secToX(segEndSec), track.width)
                        x: x1
                        width: Math.max(x2 - x1, 1)
                        height: parent.height
                        color: modelData.state === "idle" ? Theme.muted
                             : (modelData.category ? Theme.colorOf(modelData.category) : Theme.muted)
                        // no border -> segments flow continuously (no dark gaps between them)

                        MouseArea {
                            anchors.fill: parent
                            hoverEnabled: true
                            onEntered: {
                                var mins = modelData.durationMin || Math.round((segEndSec - segStartSec) / 60)
                                var cat = modelData.state === "idle" ? "idle" : (modelData.category || "?")
                                var txt = (modelData.startTime || "") + "-" + (modelData.endTime || "")
                                         + "  " + (modelData.appName || "")
                                         + "  [" + cat + "]  " + mins + "min"
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
