import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Page {
    id: root
    title: "白名单管理"

    property var viewModel
    property var picker

    ColumnLayout {
        anchors.fill: parent
        spacing: 16

        RowLayout {
            Layout.fillWidth: true
            Layout.margins: 16

            Label {
                text: "白名单应用"
                font.pixelSize: 20
                font.bold: true
            }

            Item { Layout.fillWidth: true }

            Button {
                text: picker.picking ? "请切换目标窗口(3s)..." : "+ 添加应用"
                enabled: !viewModel.loading && !picker.picking
                onClicked: picker.pick()
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
            color: "red"
            Layout.leftMargin: 16
        }

        ListView {
            id: listView
            Layout.fillWidth: true
            Layout.fillHeight: true
            model: viewModel
            clip: true
            spacing: 8

            delegate: Rectangle {
                width: listView.width - 32
                x: 16
                height: 72
                radius: 8
                color: "#f5f5f5"
                border.color: "#e0e0e0"

                RowLayout {
                    anchors.fill: parent
                    anchors.margins: 12
                    spacing: 12

                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 4

                        Label {
                            text: model.name
                            font.pixelSize: 16
                            font.bold: true
                        }
                        Label {
                            text: model.path
                            font.pixelSize: 11
                            color: "#666"
                            elide: Text.ElideMiddle
                            Layout.fillWidth: true
                        }
                    }

                    ComboBox {
                        model: ["coding", "office", "browser", "chat", "other"]
                        currentIndex: model.indexOf(model.category)
                        onActivated: (index) => {
                            viewModel.updateCategory(model.path, model[index])
                        }
                    }

                    Label {
                        text: model.todayMinutes + " min"
                        color: "#2196F3"
                        font.bold: true
                    }

                    Button {
                        text: "删除"
                        flat: true
                        onClicked: viewModel.removeApp(model.path)
                    }
                }
            }

            Label {
                visible: parent.count === 0
                anchors.centerIn: parent
                text: "暂无白名单应用，点击右上角添加"
                color: "#999"
            }
        }
    }
}
