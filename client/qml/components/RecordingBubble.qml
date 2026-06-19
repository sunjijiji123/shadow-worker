// RecordingBubble.qml - the floating recording pill (HTML .bubble-demo).
// State machine (matches ui-wireframe-v2.html section 2):
//   listening    -> wave animation + static "Listening..."
//   transcribing -> wave + scrolling transcript text
//   polishing    -> wave hidden, blue bouncing dots + "Polishing"
//   completed    -> wave hidden, green check icon + "Done"
// .bubble-demo: bg3, rule border, radius 24, padding 10x12, width 320.
//
// Layout matches HTML: left spacer (balances close btn) + centered content
// (wave/dots/check + text) + close button on the right.
//
// Animation notes:
//   - wave: animate bar HEIGHT directly (not scale). Each bar sits in a fixed
//     24px-tall Item, anchored verticalCenter, so it grows symmetrically and
//     its WIDTH (4px) never changes. Staggered 100ms per bar.
//   - polish dots: animate dot Y directly. The dot has NO anchors (anchors
//     would lock y and kill the animation); it is positioned via the animation.

import QtQuick
import QtQuick.Layouts
import ShadowWorker

Rectangle {
    id: root

    // ---- API ----
    // state: "listening" | "transcribing" | "polishing" | "completed"
    property string state: "listening"
    property string transcript: ""
    property bool showWave: state === "listening" || state === "transcribing"

    signal closeRequested()

    // .bubble-demo
    width: 320
    height: 44
    radius: 24
    color: Theme.bg3
    border.color: Theme.rule
    border.width: 1

    RowLayout {
        anchors.fill: parent
        anchors.leftMargin: 12
        anchors.rightMargin: 12
        spacing: 0

        // left spacer: balances the close button width so the content group
        // stays horizontally centered across the full bubble (HTML .float-spacer).
        Item {
            Layout.preferredWidth: 22
            Layout.fillHeight: true
        }

        // centered content: indicator (wave/dots/check) + text
        Item {
            Layout.fillWidth: true
            Layout.preferredHeight: 24

            Row {
                anchors.centerIn: parent
                spacing: 10

                // ---- left indicator (fixed 46px slot) ----
                Item {
                    width: 46
                    height: 24
                    anchors.verticalCenter: parent.verticalCenter

                    // wave: 7 bars in a Row. Each bar lives in a fixed 24px-tall
                    // slot and is anchored verticalCenter, so animating its height
                    // makes it grow symmetrically (width stays 4px).
                    Row {
                        id: wave
                        anchors.centerIn: parent
                        spacing: 3
                        visible: root.showWave

                        Repeater {
                            model: 7
                            delegate: Item {
                                width: 4
                                height: 24
                                Rectangle {
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 4
                                    radius: 2
                                    color: Theme.accent
                                    // CSS @keyframes wave: scaleY 0.4 -> 1 -> 0.4,
                                    // 1s ease-in-out, staggered 0.1s per bar.
                                    // We map scaleY 0.4..1 to height 8..22.
                                    SequentialAnimation on height {
                                        loops: Animation.Infinite
                                        PauseAnimation { duration: index * 100 }
                                        NumberAnimation {
                                            from: 8; to: 22
                                            duration: 500
                                            easing.type: Easing.InOutQuad
                                        }
                                        NumberAnimation {
                                            from: 22; to: 8
                                            duration: 500
                                            easing.type: Easing.InOutQuad
                                        }
                                    }
                                }
                            }
                        }
                    }

                    // polish-dots: 3 blue bouncing dots (polishing state only).
                    // The dot Rectangle has NO anchors so its y can be animated.
                    Row {
                        id: polishDots
                        anchors.centerIn: parent
                        spacing: 4
                        visible: root.state === "polishing"

                        Repeater {
                            model: 3
                            delegate: Item {
                                width: 6; height: 24
                                Rectangle {
                                    // centered baseline y = (24-6)/2 = 9; bounce +-4
                                    x: 0
                                    y: 9
                                    width: 6; height: 6; radius: 3
                                    color: "#3B82F6"
                                    // bounceDot: 0.55s alternate up/down, staggered
                                    SequentialAnimation on y {
                                        loops: Animation.Infinite
                                        PauseAnimation { duration: index * 120 }
                                        NumberAnimation {
                                            from: 5; to: 13
                                            duration: 275; easing.type: Easing.InOutQuad
                                        }
                                        NumberAnimation {
                                            from: 13; to: 5
                                            duration: 275; easing.type: Easing.InOutQuad
                                        }
                                    }
                                }
                            }
                        }
                    }

                    // completed check icon (green, completed state only)
                    Canvas {
                        id: checkIcon
                        anchors.centerIn: parent
                        width: 18; height: 18
                        visible: root.state === "completed"
                        onPaint: {
                            var ctx = getContext("2d")
                            ctx.reset()
                            ctx.strokeStyle = Theme.accent
                            ctx.lineWidth = 2.5
                            ctx.lineCap = "round"
                            ctx.lineJoin = "round"
                            ctx.beginPath()
                            // polyline points="20 6 9 17 4 12" scaled to 18x18
                            ctx.moveTo(15, 4.5)
                            ctx.lineTo(6.75, 12.75)
                            ctx.lineTo(3, 9)
                            ctx.stroke()
                        }
                    }
                }

                // ---- text (static or scrolling transcript) ----
                Item {
                    width: 180   // .bubble-text max-width
                    height: 20
                    anchors.verticalCenter: parent.verticalCenter
                    clip: true

                    Text {
                        id: staticText
                        anchors.centerIn: parent
                        visible: root.state !== "transcribing"
                        text: {
                            if (root.state === "polishing") return qsTr("Polishing")
                            if (root.state === "completed") return qsTr("Done")
                            return qsTr("Listening...")
                        }
                        color: Theme.ink
                        font.pixelSize: 14
                    }

                    Text {
                        id: scrollText
                        visible: root.state === "transcribing"
                        anchors.verticalCenter: parent.verticalCenter
                        x: parent.width
                        text: root.transcript
                        color: Theme.ink
                        font.pixelSize: 14
                        NumberAnimation on x {
                                            running: scrollText.visible
                                            loops: Animation.Infinite
                                            from: parent ? parent.width : 0
                                            to: -(scrollText.implicitWidth + 20)
                                            duration: 10000
                                            easing.type: Easing.Linear
                                        }
                        onVisibleChanged: if (visible) x = parent.width
                    }
                }
            }
        }

        // close button (.float-close: 22px circle, hover red)
        Rectangle {
            Layout.preferredWidth: 22
            Layout.preferredHeight: 22
            radius: 11
            color: closeMa.containsMouse ? Qt.rgba(239/255, 68/255, 68/255, 0.15) : "transparent"

            Canvas {
                anchors.centerIn: parent
                width: 14; height: 14
                onPaint: {
                    var ctx = getContext("2d")
                    ctx.reset()
                    ctx.strokeStyle = closeMa.containsMouse ? Theme.danger : Theme.muted
                    ctx.lineWidth = 2.2
                    ctx.lineCap = "round"
                    ctx.beginPath(); ctx.moveTo(12, 2); ctx.lineTo(2, 12); ctx.stroke()
                    ctx.beginPath(); ctx.moveTo(2, 2); ctx.lineTo(12, 12); ctx.stroke()
                }
            }

            MouseArea {
                id: closeMa
                anchors.fill: parent
                cursorShape: Qt.PointingHandCursor
                hoverEnabled: true
                onClicked: root.closeRequested()
            }
        }
    }
}
