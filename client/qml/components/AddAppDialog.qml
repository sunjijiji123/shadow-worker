// AddAppDialog.qml - 添加采集应用对话框（枚举可见窗口，搜索选择）。
// 打开时调用 WhitelistViewModel.listWindows() 从后端拉取当前所有可见顶层窗口，
// 用户搜索/选择后，点"添加"调 addApp(path, name, "other") 写入白名单。
//
// 【数据模型】用 QML ListModel（而非 JS 数组 + 整数 count 做 Repeater model）。
// 早期实现用 property var windows + filteredCount(int) + getWindow(index) 命令式
// 取数组元素，在 image provider 异步预览场景下 delegate 创建/绑定求值时序交错，
// getWindow(index) 偶发返回 undefined → card.win.path 抛 TypeError（日志见
// line 248）。ListModel 把数据附在 model role 上，delegate 用 required property
// 绑定，Qt 框架保证 role 在 delegate 创建时已就绪，无越界/时序问题。

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
    width: 720
    height: 560
    padding: 20
    topPadding: 20
    bottomPadding: 20
    leftPadding: 20
    rightPadding: 20

    // 全局 context property whitelistVm（不通过属性传入，避免初始化时机导致 null）。
    // 注意：对话框是 SettingsPage 的子组件，whitelistVm 是全局 context property。

    property string filterText: ""
    property string selectedPath: ""   // 当前选中窗口的 path
    property string selectedName: ""
    property bool loading: false

    // 网格列数 + 卡片尺寸
    readonly property int gridColumns: 3
    readonly property int cardGap: 12

    // 全部窗口数据（ListModel）。get/append/clear 都是 model 原生操作，
    // 变更自动通知绑定了它的视图，无需手动维护 filteredCount 之类的中间量。
    ListModel { id: allWindowsModel }

    // 过滤后的窗口数据（ListModel）。filterText 或数据源变化时由
    // applyFilter() 重建。Repeater 直接绑它做 model。
    ListModel { id: filteredModel }

    background: Rectangle {
        color: Theme.bg3
        border.color: Theme.rule
        border.width: 1
        radius: 12
    }

    // 按 filterText 把 allWindowsModel 过滤进 filteredModel。
    // 命令式重建（不用绑定），避开 Repeater 对 JS 数组的 binding loop 风险。
    function applyFilter() {
        filteredModel.clear()
        var key = filterText.toLowerCase()
        for (var i = 0; i < allWindowsModel.count; i++) {
            var w = allWindowsModel.get(i)
            if (key === "") {
                filteredModel.append(w)
                continue
            }
            var n = (w.name || "").toLowerCase()
            var t = (w.title || "").toLowerCase()
            if (n.indexOf(key) >= 0 || t.indexOf(key) >= 0) {
                filteredModel.append(w)
            }
        }
    }

    // 计算卡片宽度
    function cardWidth(containerWidth) {
        return Math.floor((containerWidth - cardGap * (gridColumns - 1)) / gridColumns)
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
            // 坑 #1/#6：自定义 TextField 的 textEdited(newText) 信号必须用
            // function(newText) 显式接收，旧式隐式参数注入在 Qt6 已废弃且
            // newText 会是 undefined → filterText 被污染 → 过滤抛 TypeError。
            onTextEdited: function(newText) {
                root.filterText = newText
                root.applyFilter()
            }
        }

        // 窗口列表（可滚动）
        Rectangle {
            id: gridContainer
            Layout.fillWidth: true
            Layout.fillHeight: true
            color: Theme.bg
            border.color: Theme.rule
            border.width: 1
            radius: 8

            // 选中索引
            property int selectedIndex: -1

            // 空状态
            Text {
                anchors.centerIn: parent
                z: 10
                visible: filteredModel.count === 0 && !root.loading
                text: allWindowsModel.count === 0
                      ? qsTr("No visible windows")
                      : qsTr("No matching windows")
                color: Theme.muted
                font.pixelSize: 13
            }

            // 加载中
            Text {
                anchors.centerIn: parent
                z: 10
                visible: root.loading && allWindowsModel.count === 0
                text: qsTr("Loading...")
                color: Theme.muted
                font.pixelSize: 13
            }

            Flickable {
                id: flick
                anchors.fill: parent
                anchors.margins: 8
                clip: true
                contentWidth: width
                // 内容高度 = 行数 × (卡高+间距)
                contentHeight: Math.ceil(Math.max(filteredModel.count, 1) / root.gridColumns)
                                * (root.cardWidth(width) * 9 / 16 + 52 + root.cardGap)
                boundsBehavior: Flickable.StopAtBounds

                Repeater {
                    model: filteredModel

                    delegate: Rectangle {
                        id: card
                        // model role 直接绑（Qt6 推荐范式，框架保证 role 就绪）。
                        required property int index
                        required property string hwnd
                        required property string path
                        required property string name
                        required property string title
                        width: root.cardWidth(flick.width)
                        height: Math.floor(width * 9 / 16) + 52
                        radius: 10
                        clip: true
                        color: Theme.bg3
                        border.color: gridContainer.selectedIndex === card.index ? Theme.accent : Theme.rule
                        border.width: gridContainer.selectedIndex === card.index ? 3 : 1

                        // 手动网格定位
                        x: (card.index % root.gridColumns) * (width + root.cardGap)
                        y: Math.floor(card.index / root.gridColumns) * (height + root.cardGap)

                        ColumnLayout {
                            anchors.fill: parent
                            anchors.margins: 6
                            spacing: 0

                            // 截图区 16:9
                            Rectangle {
                                Layout.fillWidth: true
                                Layout.preferredHeight: Math.floor(width * 9 / 16)
                                color: Theme.bg
                                clip: true

                                Image {
                                    anchors.fill: parent
                                    // hwnd 在 ListModel 里以 string 存（ListElement 不支持
                                    // 64 位 int 的完整精度， qint64 经 JS number 会丢精度），
                                    // image provider 侧 id.toLongLong() 能正确解析数字串。
                                    source: "image://winthumb/" + card.hwnd
                                            + "@" + root.cleanName(card.name)
                                    // PreserveAspectFit：后端已按原图比例缩放进
                                    // 320×180 框内（letterbox，不变形不裁剪），
                                    // 这里完整铺满预览区，边带融入卡片背景。
                                    fillMode: Image.PreserveAspectFit
                                    sourceSize.width: 320
                                    sourceSize.height: 180
                                    cache: false
                                }
                            }

                            // 信息区：首字母色块 + 应用名 + 标题
                            Row {
                                Layout.fillWidth: true
                                Layout.topMargin: 4
                                spacing: 6

                                Rectangle {
                                    width: 14; height: 14; radius: 3
                                    anchors.verticalCenter: parent.verticalCenter
                                    color: Theme.colorOf("other")
                                    Text {
                                        anchors.centerIn: parent
                                        text: root.cleanName(card.name).substring(0, 2)
                                        color: "#FFFFFF"; font.pixelSize: 8; font.weight: Font.Bold
                                    }
                                }
                                Text {
                                    text: root.cleanName(card.name)
                                    color: Theme.ink; font.pixelSize: 13; font.weight: Font.DemiBold
                                    elide: Text.ElideRight; width: parent.width - 20
                                }
                            }
                            Text {
                                text: card.title || ""
                                color: Theme.muted; font.pixelSize: 11
                                Layout.fillWidth: true
                                elide: Text.ElideRight; visible: text !== ""
                            }
                        }

                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            hoverEnabled: true
                            onClicked: {
                                gridContainer.selectedIndex = card.index
                                root.selectedPath = card.path
                                root.selectedName = root.cleanName(card.name)
                            }
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
        allWindowsModel.clear()
        filteredModel.clear()
        root.selectedPath = ""
        root.selectedName = ""
        gridContainer.selectedIndex = -1
        whitelistVm.listWindows()
    }

    // 接收窗口列表（VM 的 windowsListed 信号）。target 用全局 context property
    // whitelistVm，避免对话框创建时 whitelistVm 属性未传入导致的 null target。
    Connections {
        target: whitelistVm
        function onWindowsListed(windows, error) {
            console.log("[AddAppDialog] onWindowsListed: windows.length=" + (windows ? windows.length : "null")
                        + " error='" + error + "'")
            root.loading = false
            if (error && error !== "") {
                var win = ApplicationWindow.window
                if (win && win.toast) win.toast(qsTr("Failed to list windows: ") + error, "error")
                return
            }
            // 填充 ListModel。ListElement 不支持 qint64 完整精度，hwnd 转 string 存，
            // image provider 侧 toLongLong() 解析。
            for (var i = 0; windows && i < windows.length; i++) {
                var w = windows[i]
                allWindowsModel.append({
                    hwnd: String(w.hwnd),
                    path: w.path,
                    name: w.name,
                    title: w.title
                })
            }
            applyFilter()
            console.log("[AddAppDialog] onWindowsListed: allWindowsModel.count=" + allWindowsModel.count)
        }
    }

    // 打开前拉取并清空搜索
    onAboutToShow: {
        root.filterText = ""
        searchField.text = ""
        root.loadWindows()
    }
}
