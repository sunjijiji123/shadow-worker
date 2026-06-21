// AddAppDialog.qml - 添加采集应用对话框（枚举可见窗口，搜索选择）。
// 打开时调用 WhitelistViewModel.listWindows() 从后端拉取当前所有可见顶层窗口，
// 用户搜索/选择后，点"添加"调 addApp(path, name, "other") 写入白名单。

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Dialog {
    id: root

    title: ""
    modal: true
    closePolicy: Dialog.CloseOnEscape
    anchors.centerIn: parent
    width: 480
    height: 520
    padding: 20
    topPadding: 20
    bottomPadding: 20
    leftPadding: 20
    rightPadding: 20

    // 全局 context property whitelistVm（不通过属性传入，避免初始化时机导致 null）。
    // 注意：对话框是 SettingsPage 的子组件，whitelistVm 是全局 context property。

    // 本地状态
    property var windows: []        // 从后端拿到的全部窗口（QVariantMap 列表）
    property string filterText: ""
    property var selectedPath: ""   // 当前选中窗口的 path
    property string selectedName: ""
    property bool loading: false

    background: Rectangle {
        color: Theme.bg3
        border.color: Theme.rule
        border.width: 1
        radius: 12
    }

    // 过滤后的窗口列表（按 name 或 title 包含 filterText）
    function filteredWindows() {
        if (filterText === "") return windows
        var key = filterText.toLowerCase()
        var out = []
        for (var i = 0; i < windows.length; i++) {
            var w = windows[i]
            var n = (w.name || "").toLowerCase()
            var t = (w.title || "").toLowerCase()
            if (n.indexOf(key) >= 0 || t.indexOf(key) >= 0) {
                out.push(w)
            }
        }
        return out
    }

    // 应用名去 .exe 后缀
    function cleanName(name) {
        return (name || "").replace(/\.exe$/i, "")
    }

    contentItem: ColumnLayout {
        spacing: 12

        // 标题
        Text {
            text: qsTr("Add Tracked App")
            color: Theme.ink
            font.pixelSize: 16
            font.weight: Font.DemiBold
            Layout.fillWidth: true
        }

        Text {
            text: qsTr("Select a visible window to track its app")
            color: Theme.muted
            font.pixelSize: 12
            Layout.fillWidth: true
        }

        // 搜索框
        TextField {
            id: searchField
            Layout.fillWidth: true
            label: qsTr("Search")
            placeholder: qsTr("Filter by app name or window title")
            onTextEdited: root.filterText = newText
        }

        // 窗口列表（可滚动）
        Rectangle {
            Layout.fillWidth: true
            Layout.fillHeight: true
            color: Theme.bg
            border.color: Theme.rule
            border.width: 1
            radius: 8

            ListView {
                id: winList
                anchors.fill: parent
                anchors.margins: 1
                clip: true
                model: root.filteredWindows()
                currentIndex: -1

                // 空状态
                Text {
                    anchors.centerIn: parent
                    visible: winList.count === 0 && !root.loading
                    text: root.windows.length === 0
                          ? qsTr("No visible windows")
                          : qsTr("No matching windows")
                    color: Theme.muted
                    font.pixelSize: 13
                }

                // 加载中
                Text {
                    anchors.centerIn: parent
                    visible: root.loading && winList.count === 0
                    text: qsTr("Loading...")
                    color: Theme.muted
                    font.pixelSize: 13
                }

                delegate: Rectangle {
                    id: winRow
                    required property var modelData
                    required property int index
                    width: winList.width
                    height: 52
                    color: winList.currentIndex === index ? Theme.accentBg2
                           : (ma.containsMouse ? Theme.bg3 : "transparent")

                    RowLayout {
                        anchors.fill: parent
                        anchors.leftMargin: 12
                        anchors.rightMargin: 12
                        spacing: 10

                        // 应用图标（类别色首字母占位）
                        Rectangle {
                            width: 32
                            height: 32
                            radius: 6
                            color: Theme.colorOf("other")
                            Layout.alignment: Qt.AlignVCenter
                            Text {
                                anchors.centerIn: parent
                                text: root.cleanName(winRow.modelData.name).substring(0, 2)
                                color: "#FFFFFF"
                                font.pixelSize: 12
                                font.weight: Font.Bold
                            }
                        }

                        ColumnLayout {
                            Layout.fillWidth: true
                            spacing: 1
                            Text {
                                text: root.cleanName(winRow.modelData.name)
                                color: Theme.ink
                                font.pixelSize: 13
                                font.weight: Font.DemiBold
                                Layout.fillWidth: true
                                elide: Text.ElideRight
                            }
                            Text {
                                text: winRow.modelData.title || ""
                                color: Theme.muted
                                font.pixelSize: 11
                                Layout.fillWidth: true
                                elide: Text.ElideRight
                                visible: text !== ""
                            }
                        }
                    }

                    MouseArea {
                        id: ma
                        anchors.fill: parent
                        cursorShape: Qt.PointingHandCursor
                        hoverEnabled: true
                        onClicked: {
                            winList.currentIndex = index
                            root.selectedPath = winRow.modelData.path
                            root.selectedName = root.cleanName(winRow.modelData.name)
                        }
                    }
                }
            }
        }

        // 选中项提示
        Text {
            Layout.fillWidth: true
            text: root.selectedPath ? qsTr("Selected: ") + root.selectedName : ""
            color: Theme.accent
            font.pixelSize: 12
            visible: root.selectedPath !== ""
            elide: Text.ElideRight
        }

        // footer: 刷新 + 取消 + 添加
        RowLayout {
            Layout.fillWidth: true
            Layout.topMargin: 8
            spacing: 10

            Button {
                text: qsTr("Refresh")
                kind: "ghost"
                onClicked: root.loadWindows()
            }
            Item { Layout.fillWidth: true }
            Button {
                text: qsTr("Cancel")
                kind: "ghost"
                onClicked: root.close()
            }
            Button {
                text: qsTr("Add")
                kind: "primary"
                enabled: root.selectedPath !== ""
                onClicked: {
                    if (whitelistVm && root.selectedPath) {
                        whitelistVm.addApp(root.selectedPath, root.selectedName, "other")
                    }
                    root.close()
                }
            }
        }
    }

    // 拉取窗口列表
    function loadWindows() {
        if (!whitelistVm) return
        root.loading = true
        root.windows = []
        root.selectedPath = ""
        winList.currentIndex = -1
        whitelistVm.listWindows()
    }

    // 接收窗口列表（VM 的 windowsListed 信号）。target 用全局 context property
    // whitelistVm，避免对话框创建时 whitelistVm 属性未传入导致的 null target。
    Connections {
        target: whitelistVm
        function onWindowsListed(windows, error) {
            root.loading = false
            if (error && error !== "") {
                var win = ApplicationWindow.window
                if (win && win.toast) win.toast(qsTr("Failed to list windows: ") + error, "error")
                return
            }
            root.windows = windows
        }
    }

    // 打开前拉取并清空搜索
    onAboutToShow: {
        root.filterText = ""
        searchField.text = ""
        root.loadWindows()
    }
}
