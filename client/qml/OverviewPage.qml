// 概览页:显示从 Go 后台拿到的今日概览

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Page {
    id: root
    title: "概览"

    property var viewModel

    Component.onCompleted: viewModel.refresh()

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 24
        spacing: 16

        RowLayout {
            Layout.fillWidth: true
            Label {
                text: "概览"
                font.pixelSize: 24
                font.bold: true
            }
            Item { Layout.fillWidth: true }
            Label {
                text: viewModel.activeApp ? "当前: " + viewModel.activeApp : ""
                font.pixelSize: 12
                color: "#6b7280"
            }
            Button {
                text: "暂停"
                visible: viewModel.collectionStatus === "running"
                onClicked: viewModel.pauseCollection()
            }
            Button {
                text: "继续"
                visible: viewModel.collectionStatus === "paused"
                onClicked: viewModel.resumeCollection()
            }
            Button {
                text: "刷新"
                enabled: !viewModel.loading
                onClicked: viewModel.refresh()
            }
        }

        Label {
            visible: viewModel.error.length > 0
            text: viewModel.error
            color: "#dc2626"
            Layout.fillWidth: true
            wrapMode: Text.Wrap
        }

        Frame {
            Layout.fillWidth: true
            ColumnLayout {
                anchors.fill: parent
                spacing: 8
                Label {
                    text: "今日"
                    font.pixelSize: 14
                    color: "#6b7280"
                }
                Label {
                    text: viewModel.loading ? "加载中..." : "%1 分钟 (%2 段活动)"
                        .arg(viewModel.todayMinutes)
                        .arg(viewModel.activeSegments)
                    font.pixelSize: 28
                    font.bold: true
                }
            }
        }

        Frame {
            Layout.fillWidth: true
            ColumnLayout {
                anchors.fill: parent
                spacing: 8
                Label {
                    text: "状态"
                    font.pixelSize: 14
                    color: "#6b7280"
                }
                RowLayout {
                    spacing: 16
                    Repeater {
                        model: [
                            { label: "采集", value: viewModel.collectionStatus },
                            { label: "语音", value: viewModel.asrStatus },
                            { label: "VLM", value: viewModel.vlmStatus },
                            { label: "MCP", value: viewModel.mcpStatus }
                        ]
                        RowLayout {
                            spacing: 6
                            Rectangle {
                                width: 10; height: 10; radius: 5
                                color: modelData.value === "running" || modelData.value === "ready"
                                       ? "#10b981" : "#6b7280"
                            }
                            Label {
                                text: "%1: %2".arg(modelData.label).arg(modelData.value)
                                font.pixelSize: 12
                            }
                        }
                    }
                }
            }
        }

        Frame {
            Layout.fillWidth: true
            Layout.fillHeight: true
            ColumnLayout {
                anchors.fill: parent
                spacing: 8
                Label {
                    text: "采集应用"
                    font.pixelSize: 14
                    color: "#6b7280"
                }
                ListView {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    model: viewModel.apps
                    spacing: 4
                    delegate: RowLayout {
                        width: ListView.view.width
                        spacing: 12
                        Rectangle {
                            width: 8; height: 8; radius: 4
                            color: {
                                var c = modelData.category;
                                if (c === "coding") return "#3B82F6";
                                if (c === "office") return "#8B5CF6";
                                if (c === "browser") return "#F59E0B";
                                if (c === "chat") return "#10B981";
                                return "#6B7280";
                            }
                        }
                        Label {
                            text: modelData.name
                            font.pixelSize: 13
                            font.bold: true
                        }
                        Label {
                            text: modelData.category
                            font.pixelSize: 12
                            color: "#6b7280"
                        }
                        Item { Layout.fillWidth: true }
                        Label {
                            text: "%1m".arg(modelData.todayMinutes)
                            font.pixelSize: 13
                            color: "#6b7280"
                        }
                    }
                }
            }
        }
    }
}
