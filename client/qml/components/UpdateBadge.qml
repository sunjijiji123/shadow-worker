// UpdateBadge.qml - 标题栏的更新徽标。
//
// 根据 state 显示不同形态：
//   idle      → 绿色 "Update" 胶囊 + 右上角红点（发现新版本，待用户查看）
//   downloading → 圆环进度条 + 百分比（下载中）
//   ready     → 绿色 "Restart to Update" 胶囊（已下载，可立即安装）
//   failed    → 红色感叹号胶囊（下载失败，点击重试）
//
// 胶囊本身不触发任何业务逻辑，只发 clicked 信号，由宿主（TitleBar）决定
// 打开详情弹窗还是直接拉起安装。
//
// 圆环用 Canvas 绘制：背景灰圈 + 前景绿色弧，圆心显示百分比数字。

import QtQuick
import ShadowWorker

Item {
    id: root

    // idle | downloading | ready | failed
    property string badgeState: "idle"
    property int progress: 0       // 0-100，仅 downloading 状态用

    signal clicked()

    // 不同状态的高度/宽度差异较大，用隐式尺寸让布局自适应。
    // 胶囊形态（idle/ready/failed）：宽 = 文本+padding，高 = 24
    // 圆环形态（downloading）：固定 24x24
    implicitHeight: 24
    implicitWidth: badgeState === "downloading" ? 30 : (label.implicitWidth + 22)

    // 胶囊背景（idle/ready/failed 三态共用，颜色随状态变）
    Rectangle {
        id: capsule
        anchors.fill: parent
        visible: root.badgeState !== "downloading"
        radius: 12
        color: {
            if (root.badgeState === "ready") return Theme.accent       // 绿色实心
            if (root.badgeState === "failed") return Qt.rgba(239/255, 68/255, 68/255, 0.18) // 红色淡底
            return Qt.rgba(52/255, 211/255, 153/255, 0.18)            // idle 绿色淡底
        }
        border.color: {
            if (root.badgeState === "ready") return Theme.accent
            if (root.badgeState === "failed") return Qt.rgba(239/255, 68/255, 68/255, 0.45)
            return Qt.rgba(52/255, 211/255, 153/255, 0.5)
        }
        border.width: 1

        // 红点角标（仅 idle 态：提示"有新内容待查看"）
        Rectangle {
            visible: root.badgeState === "idle"
            anchors.top: parent.top
            anchors.right: parent.right
            anchors.topMargin: 2
            anchors.rightMargin: 2
            width: 6; height: 6
            radius: 3
            color: "#ef4444"
            border.color: Theme.bg2
            border.width: 1
        }

        // 文本
        Text {
            id: label
            anchors.centerIn: parent
            font.pixelSize: Theme.fontTiny   // 11px
            font.weight: Font.DemiBold
            text: {
                if (root.badgeState === "ready") return qsTr("Restart to Update")
                if (root.badgeState === "failed") return qsTr("Update failed")
                return qsTr("Update")
            }
            color: {
                if (root.badgeState === "ready") return "#000000"
                if (root.badgeState === "failed") return "#ef4444"
                return Qt.rgba(52/255, 211/255, 153/255, 1.0)
            }
        }
    }

    // 圆环进度（downloading 态）
    Item {
        id: ring
        anchors.fill: parent
        visible: root.badgeState === "downloading"

        Canvas {
            id: canvas
            anchors.centerIn: parent
            width: 24; height: 24
            // progress 变化时重绘弧线（注意：Canvas 本身没有 progress 属性，
            // 必须监听 root 的 progressChanged 信号，不能写 onProgressChanged）
            Connections {
                target: root
                function onProgressChanged() { canvas.requestPaint() }
            }

            onPaint: {
                var ctx = getContext("2d")
                ctx.reset()
                ctx.beginPath()
                ctx.lineWidth = 2
                ctx.strokeStyle = Theme.rule   // 背景灰圈
                ctx.arc(12, 12, 9, 0, 2 * Math.PI)
                ctx.stroke()

                // 前景弧（从 12 点顺时针画 progress%）
                ctx.beginPath()
                ctx.lineWidth = 2
                ctx.strokeStyle = Theme.accent
                ctx.lineCap = "round"
                var start = -Math.PI / 2
                var end = start + 2 * Math.PI * (root.progress / 100)
                ctx.arc(12, 12, 9, start, end)
                ctx.stroke()
            }
        }

        Text {
            anchors.centerIn: parent
            text: root.progress + "%"
            font.pixelSize: 8
            font.weight: Font.DemiBold
            color: Theme.ink
        }
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        hoverEnabled: true
        onClicked: root.clicked()
    }
}
