import QtQuick
import QtQuick.Controls

// Bubble 是语音识别结果的气泡提示,定位在屏幕右下角。
Window {
    id: root
    flags: Qt.FramelessWindowHint | Qt.WindowStaysOnTopHint | Qt.Tool
    color: "transparent"
    width: 360
    height: 120
    visible: false

    property alias text: label.text
    property bool recording: false

    function show(msg) {
        text = msg;
        visible = true;
        fadeTimer.restart();
    }

    function showRecording() {
        recording = true;
        visible = true;
        fadeTimer.stop();
    }

    function hide() {
        recording = false;
        visible = false;
    }

    Timer {
        id: fadeTimer
        interval: 5000
        onTriggered: root.hide()
    }

    Rectangle {
        anchors.fill: parent
        anchors.margins: 8
        radius: 12
        color: recording ? "#E53935" : "#263238"
        border.color: "#455A64"

        Column {
            anchors.fill: parent
            anchors.margins: 12
            spacing: 6

            Label {
                text: recording ? "正在录音..." : "识别结果"
                color: "#90A4AE"
                font.pixelSize: 12
            }

            Label {
                id: label
                width: parent.width
                color: "white"
                font.pixelSize: 14
                wrapMode: Text.Wrap
                maximumLineCount: 3
                elide: Text.ElideRight
            }
        }
    }
}
