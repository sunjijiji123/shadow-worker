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
    // idle(Ready) 也显示装饰性波形，和 listening 无音频输入时一样的脉动效果。
    // transcribing 不显示波形——改用转圈 loading（见 transcribing spinner）。
    property bool showWave: state === "listening" || state === "idle"
    // live 16-band spectrum from the backend (array of floats [0..1]).
    // When non-empty, the 7 visible bars are driven by mapped bands; otherwise
    // a decorative staggered animation plays (idle / no capture).
    property var bands: []
    // true when we have live spectrum data to render
    readonly property bool hasLive: bands !== undefined && bands.length >= 16

    // Map the 16 backend bands down to the 7 visible bars: pick 7 evenly
    // spaced bands and scale each to a 0..1 weight. The backend already applies
    // a bell-ish shape via FFT + normalization, so we just subsample.
    readonly property var liveWeights: {
        if (!hasLive) return [0,0,0,0,0,0,0]
        var out = []
        // sample 7 of the 16 bands (indices 2..14, step ~2)
        var idx = [2, 4, 6, 8, 10, 12, 14]
        for (var i = 0; i < 7; i++) {
            var v = bands[idx[i]] || 0
            out.push(Math.max(0, Math.min(1, v)))
        }
        return out
    }

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
                                    id: waveBar
                                    anchors.horizontalCenter: parent.horizontalCenter
                                    anchors.verticalCenter: parent.verticalCenter
                                    width: 4
                                    radius: 2
                                    color: Theme.accent

                                    // LIVE spectrum: bar height follows the mapped band
                                    // weight (0..1) from the backend FFT. Each bar already
                                    // has independent frequency energy, so no jitter needed.
                                    // Map weight 0..1 -> height 3..22px.
                                    readonly property int liveHeight:
                                        Math.max(3, Math.min(22, Math.round(3 + (root.liveWeights[index] || 0) * 19)))
                                    // DECORATIVE (no live data): staggered pulse so the
                                    // wave isn't dead when idle.
                                    readonly property real idleHeight: waveAnim.running ? waveAnim.currentHeight : 6
                                    height: root.hasLive ? liveHeight : idleHeight

                                    property real currentHeight: 8
                                    SequentialAnimation {
                                        id: waveAnim
                                        running: root.showWave && !root.hasLive
                                        loops: Animation.Infinite
                                        PauseAnimation { duration: index * 100 }
                                        NumberAnimation {
                                            target: waveBar
                                            property: "currentHeight"
                                            from: 8; to: 22
                                            duration: 500
                                            easing.type: Easing.InOutQuad
                                        }
                                        NumberAnimation {
                                            target: waveBar
                                            property: "currentHeight"
                                            from: 22; to: 8
                                            duration: 500
                                            easing.type: Easing.InOutQuad
                                        }
                                    }
                                }
                            }
                        }
                    }

                    // transcribing spinner: 转写期间的转圈 loading（accent 色，
                    // 24px ring 缺口，0.8s 一圈）。借鉴 ResultBubble 的 polishing
                    // overlay spinner 样式。
                    Item {
                        id: transcribingSpinner
                        width: 24; height: 24
                        anchors.centerIn: parent
                        visible: root.state === "transcribing"

                        RotationAnimation on rotation {
                            running: transcribingSpinner.visible
                            loops: Animation.Infinite
                            from: 0; to: 360; duration: 800
                        }

                        Canvas {
                            anchors.fill: parent
                            onPaint: {
                                var ctx = getContext("2d")
                                ctx.reset()
                                // 底圈：暗色完整圆（中心 12,12，半径 9）
                                ctx.strokeStyle = Theme.rule
                                ctx.lineWidth = 2.5
                                ctx.beginPath()
                                ctx.arc(12, 12, 9, 0, 2 * Math.PI)
                                ctx.stroke()
                                // 上层：accent 色 1/4 弧（缺口），随父 Item 旋转
                                ctx.strokeStyle = Theme.accent
                                ctx.beginPath()
                                ctx.arc(12, 12, 9, 0, 0.5 * Math.PI)
                                ctx.stroke()
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

                    // completed check icon (accent, 识别成功)
                    Canvas {
                        id: checkIcon
                        anchors.centerIn: parent
                        width: 24; height: 24
                        visible: root.state === "completed"
                        onPaint: {
                            var ctx = getContext("2d")
                            ctx.reset()
                            ctx.strokeStyle = Theme.accent
                            ctx.lineWidth = 3
                            ctx.lineCap = "round"
                            ctx.lineJoin = "round"
                            ctx.beginPath()
                            // 对勾，缩放到 24x24（原 18x18 的 ×4/3）
                            ctx.moveTo(20, 6)
                            ctx.lineTo(9, 17)
                            ctx.lineTo(4, 12)
                            ctx.stroke()
                        }
                    }

                    // error icon (danger/red, 识别失败)
                    Canvas {
                        id: errorIcon
                        anchors.centerIn: parent
                        width: 24; height: 24
                        visible: root.state === "error"
                        onPaint: {
                            var ctx = getContext("2d")
                            ctx.reset()
                            ctx.strokeStyle = Theme.danger
                            ctx.lineWidth = 3
                            ctx.lineCap = "round"
                            ctx.beginPath(); ctx.moveTo(7, 7); ctx.lineTo(17, 17); ctx.stroke()
                            ctx.beginPath(); ctx.moveTo(17, 7); ctx.lineTo(7, 17); ctx.stroke()
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
                        // transcribing 也用 staticText 显示"正在识别"（cloud ASR
                        // 一次性返回结果，不是流式，所以转写期间 transcript 为空，
                        // 滚动文字没意义，改用静态 loading 提示）。
                        visible: root.state !== "transcribing" || root.transcript === ""
                        text: {
                            if (root.state === "idle") return qsTr("Ready")
                            if (root.state === "transcribing") return qsTr("Recognizing...")
                            if (root.state === "polishing") return qsTr("Polishing")
                            if (root.state === "completed") return qsTr("Done")
                            if (root.state === "error") return qsTr("Failed")
                            return qsTr("Listening...")
                        }
                        color: Theme.ink
                        font.pixelSize: 14
                    }

                    Text {
                        id: scrollText
                        // 仅在转写且有流式 transcript 时滚动显示（当前 cloud ASR
                        // 一次性返回，transcript 为空，走 staticText 的"正在识别"）。
                        visible: root.state === "transcribing" && root.transcript !== ""
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
