import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Page {
    id: root
    title: "时间线"

    property var viewModel

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        RowLayout {
            Layout.fillWidth: true
            spacing: 8

            Label { text: "日期:" }
            TextField {
                id: dateField
                text: viewModel ? viewModel.date : ""
                onEditingFinished: {
                    if (viewModel)
                        viewModel.date = text;
                }
                Layout.minimumWidth: 120
            }
            Button {
                text: "刷新"
                onClicked: {
                    if (viewModel)
                        viewModel.refresh();
                }
            }
            BusyIndicator {
                running: viewModel ? viewModel.loading : false
                implicitWidth: 24
                implicitHeight: 24
            }
            Label {
                visible: viewModel && viewModel.error.length > 0
                text: viewModel ? viewModel.error : ""
                color: "red"
                Layout.fillWidth: true
            }
        }

        Label {
            text: "活动段"
            font.bold: true
            font.pixelSize: 16
        }

        ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true

            Column {
                width: parent.width
                spacing: 6

                Repeater {
                    model: viewModel ? viewModel.segments : []

                    Rectangle {
                        width: parent.width
                        height: 56
                        color: index % 2 === 0 ? "#f5f5f5" : "white"
                        radius: 4

                        RowLayout {
                            anchors.fill: parent
                            anchors.margins: 8
                            spacing: 10

                            ColumnLayout {
                                spacing: 2
                                Label {
                                    text: modelData.startTime + " - " + modelData.endTime
                                    font.bold: true
                                }
                                Label {
                                    text: (modelData.appName || "未知") + " / " + (modelData.category || "其他")
                                    font.pixelSize: 12
                                    color: "#555"
                                }
                                Label {
                                    text: modelData.windowTitle || ""
                                    font.pixelSize: 11
                                    color: "#888"
                                    elide: Text.ElideRight
                                    Layout.maximumWidth: 400
                                }
                            }

                            Item { Layout.fillWidth: true }

                            Label {
                                text: formatDuration(modelData.durationSec)
                                font.pixelSize: 12
                                color: "#333"
                            }
                        }
                    }
                }
            }
        }

        Label {
            text: "事件"
            font.bold: true
            font.pixelSize: 16
        }

        ScrollView {
            Layout.fillWidth: true
            Layout.preferredHeight: 160
            clip: true

            Column {
                width: parent.width
                spacing: 4

                Repeater {
                    model: viewModel ? viewModel.events : []

                    RowLayout {
                        width: parent.width
                        spacing: 8

                        Label {
                            text: modelData.time
                            font.pixelSize: 12
                            color: "#666"
                        }
                        Label {
                            text: "[" + (modelData.type || "") + "]"
                            font.pixelSize: 12
                            color: "#0066cc"
                        }
                        Label {
                            text: modelData.text || ""
                            font.pixelSize: 12
                            elide: Text.ElideRight
                            Layout.fillWidth: true
                        }
                    }
                }
            }
        }
    }

    function formatDuration(seconds) {
        var m = Math.floor(seconds / 60);
        var s = seconds % 60;
        return m + "分" + (s < 10 ? "0" + s : s) + "秒";
    }

    Component.onCompleted: {
        if (viewModel)
            viewModel.refresh();
    }
}
