import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

ApplicationWindow {
    id: mainWindow
    visible: true
    width: 900
    height: 600
    title: "Shadow Worker"

    property bool asrRecording: false

    header: ToolBar {
        RowLayout {
            anchors.fill: parent
            anchors.margins: 8

            Label {
                text: "Shadow Worker"
                font.pixelSize: 18
                font.bold: true
            }

            Item { Layout.fillWidth: true }

            Button {
                text: "概览"
                flat: stackView.currentItem && stackView.currentItem.title === "概览"
                onClicked: stackView.replace(overviewPageComponent)
            }
            Button {
                text: "白名单"
                flat: stackView.currentItem && stackView.currentItem.title === "白名单管理"
                onClicked: stackView.replace(whitelistPageComponent)
            }
            Button {
                text: "时间线"
                flat: stackView.currentItem && stackView.currentItem.title === "时间线"
                onClicked: stackView.replace(timelinePageComponent)
            }
            Button {
                text: "设置"
                flat: stackView.currentItem && stackView.currentItem.title === "设置"
                onClicked: stackView.replace(settingsPageComponent)
            }
        }
    }

    StackView {
        id: stackView
        anchors.fill: parent
        initialItem: overviewPageComponent
    }

    Component {
        id: overviewPageComponent
        OverviewPage {
            viewModel: overviewVm
        }
    }

    Component {
        id: whitelistPageComponent
        WhitelistPage {
            viewModel: whitelistVm
            picker: windowPicker
        }
    }

    Component {
        id: timelinePageComponent
        TimelinePage {
            viewModel: timelineVm
        }
    }

    Component {
        id: settingsPageComponent
        SettingsPage {
            viewModel: settingsVm
        }
    }

    // 全局热键: 录音/截图
    Connections {
        target: globalHotkey
        function onActivatedWithName(name) {
            if (name === "record") {
                if (asrRecording) stopRecording(); else startRecording();
            } else if (name === "screenshot") {
                bubble.show("触发 VLM 截图理解...");
            }
        }
    }

    Connections {
        target: settingsVm
        function onHotkeyRecordChanged() { updateHotkeys(); }
        function onHotkeyScreenshotChanged() { updateHotkeys(); }
    }

    function updateHotkeys() {
        globalHotkey.unregisterAll();
        if (settingsVm && settingsVm.hotkeyRecord !== "")
            globalHotkey.registerShortcut(settingsVm.hotkeyRecord, "record");
        if (settingsVm && settingsVm.hotkeyScreenshot !== "")
            globalHotkey.registerShortcut(settingsVm.hotkeyScreenshot, "screenshot");
    }

    function startRecording() {
        asrRecording = true;
        bubble.showRecording();
        asrClient.start();
        audioRecorder.startRecording();
    }

    function stopRecording() {
        asrRecording = false;
        audioRecorder.stopRecording();
        asrClient.finish();
        bubble.text = "识别中...";
        bubble.show(bubble.text);
    }

    // 定时把录音数据喂给 ASR
    Timer {
        interval: 200
        running: asrRecording
        repeat: true
        onTriggered: {
            var pcm = audioRecorder.takeAudioData();
            if (pcm.length > 0)
                asrClient.feed(pcm);
        }
    }

    Connections {
        target: asrClient
        function onResultReady(text) {
            bubble.show(text);
        }
        function onErrorChanged() {
            if (asrClient.error.length > 0)
                bubble.show("ASR 错误: " + asrClient.error);
        }
    }

    Loader {
        id: bubbleLoader
        active: true
        sourceComponent: Bubble {
            id: bubble
            x: Screen.desktopAvailableWidth - width - 20
            y: Screen.desktopAvailableHeight - height - 20
        }
    }

    property var bubble: bubbleLoader.item

    Component.onCompleted: {
        if (autostartMode)
            showMinimized();
        updateHotkeys();
        whitelistVm.refresh();
    }
}
