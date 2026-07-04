// UpdateDialog.qml - 更新详情弹窗。
//
// 展示版本号/发布时间/大小 + Markdown 格式的更新日志（Qt6 Text.MarkdownText 原生渲染）。
// 底部按钮随下载状态变化：
//   idle      → 立即更新(primary) + 稍后(ghost)
//   downloading → 取消(ghost) + 进度文字
//   ready     → 立即安装并重启(primary，调 launchInstaller)
//   failed    → 重试(primary) + 错误信息
//
// 数据来自 updateVm context property。打开/关闭由宿主控制（main.qml）。
// 挂在 main.qml 顶层，避开 StackLayout 可见性坑（不可见页的 modal 会卡死窗口）。

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Dialog {
    id: root

    modal: true
    closePolicy: Dialog.CloseOnEscape | Dialog.CloseOnPressOutside
    anchors.centerIn: parent
    width: 520
    height: 480
    padding: 0

    property string downloadState: updateVm ? updateVm.downloadState : "idle"
    property int downloadProgress: updateVm ? updateVm.downloadProgress : 0

    background: Rectangle {
        color: Theme.bg3
        border.color: Theme.rule
        border.width: 1
        radius: 12
    }

    contentItem: ColumnLayout {
        spacing: 0

        // ---- 头部：版本号 + 发布时间 + 大小 ----
        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 72
            color: "transparent"

            ColumnLayout {
                anchors.fill: parent
                anchors.leftMargin: 20
                anchors.rightMargin: 20
                anchors.topMargin: 16
                anchors.bottomMargin: 16
                spacing: 4

                Text {
                    text: updateVm ? qsTr("v%1").arg(updateVm.latestVersion) : ""
                    color: Theme.ink
                    font.pixelSize: 18
                    font.weight: Font.DemiBold
                }
                Text {
                    text: updateVm
                          ? qsTr("Released %1 · %2").arg(updateVm.publishedAt).arg(updateVm.packageSizeText)
                          : ""
                    color: Theme.muted
                    font.pixelSize: 12
                }
            }

            // 底部分隔线
            Rectangle {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.bottom: parent.bottom
                height: 1
                color: Theme.rule
            }
        }

        // ---- 主体：Markdown 更新日志 ----
        // 用 Flickable 而非 ScrollView：ScrollView 的内部 flickable 会让
        // parent.width 解析成内容宽（循环依赖），导致 wrapMode 失效、文字溢出。
        // Flickable 的 parent 就是自身，width: parent.width 取的是真实视口宽。
        // 关键三点：contentWidth=width（禁水平滚动）、Text.width 绑定视口宽
        // （让 wrapMode 有换行基准）、contentHeight 跟随 Text 高度（垂直滚动）。
        Flickable {
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true
            contentWidth: width              // 内容宽 = 视口宽，永不出现水平滚动条
            contentHeight: bodyText.height   // 垂直可滚动，跟随 Markdown 实际渲染高度
            boundsBehavior: Flickable.StopAtBounds

            ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }
            // 不挂 ScrollBar.horizontal —— contentWidth==width 时它本就不会出现

            Text {
                id: bodyText
                // Qt6 原生支持 GitHub Flavored Markdown 渲染
                textFormat: Text.MarkdownText
                text: updateVm ? updateVm.changelog : ""
                color: Theme.ink
                font.pixelSize: 13
                wrapMode: Text.Wrap          // 配合 width 绑定，超长行在视口宽处换行
                width: parent.width          // 绑定 Flickable 视口宽，wrapMode 才有基准
                topPadding: 16
                bottomPadding: 16
                leftPadding: 20
                rightPadding: 20
                onLinkActivated: function(link) {
                    Qt.openUrlExternally(link)
                }
            }
        }

        // ---- 错误信息（failed 态） ----
        Text {
            visible: root.downloadState === "failed" && updateVm && updateVm.error !== ""
            Layout.fillWidth: true
            Layout.leftMargin: 20
            Layout.rightMargin: 20
            Layout.topMargin: 8
            text: updateVm ? updateVm.error : ""
            color: "#ef4444"
            font.pixelSize: 12
            wrapMode: Text.Wrap
        }

        // ---- 底部按钮栏 ----
        Rectangle {
            Layout.fillWidth: true
            Layout.preferredHeight: 56
            color: "transparent"

            Rectangle {
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                height: 1
                color: Theme.rule
            }

            RowLayout {
                anchors.fill: parent
                anchors.leftMargin: 20
                anchors.rightMargin: 20
                spacing: 10

                // 下载进度文字（downloading 态）
                Text {
                    visible: root.downloadState === "downloading"
                    text: qsTr("Downloading... %1%").arg(root.downloadProgress)
                    color: Theme.muted
                    font.pixelSize: 12
                    Layout.fillWidth: true
                }

                Item {
                    visible: root.downloadState !== "downloading"
                    Layout.fillWidth: true
                }

                // 取消按钮（downloading 态）
                Button {
                    visible: root.downloadState === "downloading"
                    text: qsTr("Cancel")
                    kind: "ghost"
                    onClicked: if (updateVm) updateVm.cancelDownload()
                }

                // 稍后按钮（idle/failed 态：关闭弹窗）
                Button {
                    visible: root.downloadState === "idle" || root.downloadState === "failed"
                    text: qsTr("Later")
                    kind: "ghost"
                    onClicked: root.close()
                }

                // 立即更新按钮（idle 态：开始下载）
                Button {
                    visible: root.downloadState === "idle"
                    text: qsTr("Update Now")
                    kind: "primary"
                    onClicked: if (updateVm) updateVm.startDownload()
                }

                // 重试按钮（failed 态：重新下载）
                Button {
                    visible: root.downloadState === "failed"
                    text: qsTr("Retry")
                    kind: "primary"
                    onClicked: if (updateVm) updateVm.startDownload()
                }

                // 立即安装并重启（ready 态：拉起安装包）
                Button {
                    visible: root.downloadState === "ready"
                    text: qsTr("Install & Restart")
                    kind: "primary"
                    onClicked: {
                        if (updateVm) {
                            updateVm.launchInstaller()
                            root.close()
                        }
                    }
                }
            }
        }
    }
}
