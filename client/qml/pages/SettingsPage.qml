// SettingsPage.qml - settings page (v2).
// 6 tabs. Voice Input tab fully implemented; others are placeholders.
// Matches ui-wireframe-v2.html settings section.

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var viewModel: null
    property var whitelistViewModel: null
    property var windowPicker: null

    property string activeTab: "voice"

    // ---- Voice tab local state (will bind to viewModel later) ----
    property bool recordEnabled: true
    property string recordMode: "hold"   // hold | press
    property string asrActiveModel: "xiaomi"
    property string asrModelType: "cloud"  // cloud | local (from active chip)
    property int micTestLevel: 0

    Flickable {
        anchors.fill: parent
        anchors.margins: 20
        contentWidth: width
        contentHeight: contentCol.implicitHeight
        flickableDirection: Flickable.VerticalFlick
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

        ColumnLayout {
            id: contentCol
            width: parent.width
            spacing: 16
            // bottom spacer so SaveBar doesn't cover the last card
            property int saveBarH: 64

            Text {
                text: qsTr("Settings")
                color: Theme.ink
                font.pixelSize: Theme.fontTitle
                font.weight: Font.DemiBold
            }

            // ---- tab-strip (6 tabs) ----
            Row {
                Layout.fillWidth: true
                spacing: 24

                Repeater {
                    model: [
                        { tab: "voice",    label: qsTr("Voice Input") },
                        { tab: "apps",     label: qsTr("Behavior Capture") },
                        { tab: "vision",   label: qsTr("Vision") },
                        { tab: "polish",   label: qsTr("Text Polish") },
                        { tab: "personal", label: qsTr("Personal Prompts") },
                        { tab: "tools",    label: qsTr("Quick Tools") }
                    ]
                    delegate: ColumnLayout {
                        required property var modelData
                        spacing: 6

                        Text {
                            text: modelData.label
                            color: activeTab === modelData.tab ? Theme.accent : Theme.muted
                            font.pixelSize: 14
                            MouseArea {
                                anchors.fill: parent
                                cursorShape: Qt.PointingHandCursor
                                onClicked: activeTab = modelData.tab
                            }
                        }
                        Rectangle {
                            width: tabLbl2.implicitWidth + 8
                            height: 2
                            color: activeTab === modelData.tab ? Theme.accent : "transparent"
                            Text { id: tabLbl2; visible: false; text: modelData.label; font.pixelSize: 14 }
                        }
                    }
                }
            }

            Rectangle { Layout.fillWidth: true; height: 1; color: Theme.rule }

            // ================================================================
            // VOICE INPUT TAB
            // ================================================================

            // ---- Card 1: Record Hotkey ----
            Card {
                Layout.fillWidth: true
                visible: activeTab === "voice"
                title: qsTr("Record Hotkey")
                description: qsTr("Enable global hotkey to trigger voice input. Conflicts are auto-detected.")
                headerExtra: [
                    Toggle {
                        checked: recordEnabled
                        onToggled: recordEnabled = checked
                    }
                ]

                // separator + mode area
                Rectangle { Layout.fillWidth: true; height: 1; color: Theme.rule }

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 16

                    // radio group: hold / press
                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 10

                        Radio {
                            text: qsTr("Hold to Record")
                            checked: recordMode === "hold"
                            onClicked: recordMode = "hold"
                        }
                        Radio {
                            text: qsTr("Press to Record")
                            checked: recordMode === "press"
                            onClicked: recordMode = "press"
                        }
                    }

                    // modifier + key form row
                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 16

                        SelectBox {
                            Layout.fillWidth: true
                            label: qsTr("Modifier")
                            options: [qsTr("None"), "Ctrl", "Alt", "Win", "Ctrl + Shift"]
                            currentIndex: 4
                        }

                        TextField {
                            Layout.fillWidth: true
                            label: qsTr("Key")
                            text: "R"
                        }
                    }
                }
            }

            // ---- Card 2: ASR Model Service ----
            Card {
                Layout.fillWidth: true
                visible: activeTab === "voice"
                title: qsTr("ASR Model Service")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 16

                    // model chips
                    ModelChipGroup {
                        Layout.fillWidth: true
                        activeKey: asrActiveModel
                        chips: [
                            { key: "xiaomi",   label: "Xiaomi-ASR",   type: "cloud", deletable: true },
                            { key: "deepseek", label: "DeepSeek-ASR", type: "cloud", deletable: true },
                            { key: "local",    label: "Local-whisper", type: "local", deletable: true }
                        ]
                        onChipClicked: function(key) {
                            asrActiveModel = key
                            if (key === "local") asrModelType = "local"
                            else asrModelType = "cloud"
                        }
                        onAddClicked: addModelDialog.open()
                    }

                    Text {
                        text: qsTr("Switch models to view or edit config. Add multiple for comparison.")
                        color: Theme.muted
                        font.pixelSize: 13
                        wrapMode: Text.WordWrap
                        Layout.fillWidth: true
                    }

                    // ---- Cloud fields (visible when asrModelType === "cloud") ----
                    GridLayout {
                        Layout.fillWidth: true
                        visible: asrModelType === "cloud"
                        columns: 2
                        rowSpacing: 12
                        columnSpacing: 16

                        TextField {
                            label: qsTr("Vendor Name")
                            text: "Xiaomi MIMO"
                            readOnly: true
                            Layout.fillWidth: true
                        }
                        TextField {
                            label: qsTr("Base URL")
                            text: "wss://speech.xiaomi.com/v1"
                            Layout.fillWidth: true
                        }
                        TextField {
                            label: qsTr("Model")
                            text: "xiaomi-asr"
                            Layout.fillWidth: true
                        }
                        SelectBox {
                            label: qsTr("Language")
                            options: [qsTr("Chinese (zh)"), qsTr("English (en)"), qsTr("Japanese (ja)"), qsTr("Zh+En mixed"), qsTr("Auto-detect")]
                            Layout.fillWidth: true
                        }
                        SelectBox {
                            label: qsTr("API Format")
                            options: ["OpenAI", "Anthropic messages"]
                            Layout.fillWidth: true
                        }
                        SelectBox {
                            label: qsTr("Auth Method")
                            options: ["Bearer", "api-key header", qsTr("No auth")]
                            Layout.fillWidth: true
                        }

                        // API Key (span full width)
                        TextField {
                            label: qsTr("API Key")
                            text: "sk-xxxxxxxx"
                            isPassword: true
                            Layout.columnSpan: 2
                            Layout.fillWidth: true
                        }
                    }

                    // ---- Local fields (visible when asrModelType === "local") ----
                    ColumnLayout {
                        Layout.fillWidth: true
                        visible: asrModelType === "local"
                        spacing: 12

                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 8

                            TextField {
                                Layout.fillWidth: true
                                label: qsTr("Model Path")
                                text: "C:\\Models\\ggml-base.bin"
                            }
                            Button {
                                text: qsTr("Browse...")
                                kind: "ghost"
                                Layout.alignment: Qt.AlignBottom
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 16

                            TextField {
                                Layout.fillWidth: true
                                label: qsTr("Model Name (auto from path)")
                                text: "ggml-base"
                                readOnly: true
                            }
                            SelectBox {
                                Layout.fillWidth: true
                                label: qsTr("Language")
                                options: [qsTr("Chinese (zh)"), qsTr("English (en)"), qsTr("Japanese (ja)"), qsTr("Zh+En mixed"), qsTr("Auto-detect")]
                            }
                        }

                        Text {
                            text: qsTr("Local whisper.cpp model. Language is used for decode hint and initial token.")
                            color: Theme.muted
                            font.pixelSize: 13
                            wrapMode: Text.WordWrap
                            Layout.fillWidth: true
                        }
                    }

                    // test connection button
                    Row {
                        spacing: 8
                        Button {
                            text: qsTr("Test Connection")
                            kind: "primary"
                        }
                        Text {
                            text: qsTr("86 ms latency")
                            color: Theme.muted
                            font.pixelSize: 12
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                }
            }

            // ---- Card 3: Audio Device ----
            Card {
                Layout.fillWidth: true
                visible: activeTab === "voice"
                title: qsTr("Audio Device")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 12

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 16

                        SelectBox {
                            Layout.fillWidth: true
                            Layout.preferredWidth: 2
                            label: qsTr("Input Device")
                            options: [qsTr("Default Microphone"), qsTr("Microphone (Realtek)"), qsTr("Microphone (USB)")]
                        }

                        // mic test button (green pill, HTML .mic-test-btn)
                        MicTestButton {
                            Layout.alignment: Qt.AlignBottom
                            onClicked: {
                                // TODO: wire to audio recorder for real level
                            }
                        }
                    }

                    // volume bar (simulated level)
                    VolumeBar {
                        Layout.fillWidth: true
                        level: micTestLevel
                    }
                }
            }

            // ================================================================
            // OTHER TAB PLACEHOLDERS (kept as-is, implemented later)
            // ================================================================

            Card {
                Layout.fillWidth: true
                visible: activeTab === "apps"
                title: qsTr("Tracked Apps (Whitelist)")
                description: qsTr("Card list + category chips + add (window picker)")
                PlaceholderLabel { text: qsTr("Pending") }
            }
            Card {
                Layout.fillWidth: true
                visible: activeTab === "apps"
                title: qsTr("Capture Rules")
                PlaceholderLabel { text: qsTr("Pending") }
            }
            Card {
                Layout.fillWidth: true
                visible: activeTab === "vision"
                title: qsTr("VLM / Model Service / Range / Params")
                PlaceholderLabel { text: qsTr("Pending") }
            }
            Card {
                Layout.fillWidth: true
                visible: activeTab === "polish"
                title: qsTr("Auto Polish / LLM Model / Polish Prompt")
                PlaceholderLabel { text: qsTr("Pending") }
            }
            Card {
                Layout.fillWidth: true
                visible: activeTab === "personal"
                title: qsTr("Quick Inject / Personal Prompt List")
                PlaceholderLabel { text: qsTr("Pending") }
            }
            Card {
                Layout.fillWidth: true
                visible: activeTab === "tools"
                title: qsTr("Screenshot / Data Management")
                PlaceholderLabel { text: qsTr("Pending") }
            }

            // bottom spacer (SaveBar height) so content isn't hidden behind it
            Item { Layout.fillWidth: true; Layout.preferredHeight: 70 }
        }
    }

    // bottom save bar (visible on settings page)
    SaveBar {
        onSaveRequested: {
            // ApplicationWindow.window gives the root ApplicationWindow (from QtQuick.Controls)
            var win = ApplicationWindow.window
            if (win && win.toast) win.toast(qsTr("Settings saved"))
        }
    }

    // add model dialog (shown when "+ Add Model" chip clicked)
    AddModelDialog {
        id: addModelDialog
        parent: Overlay.overlay
        onSaved: function(name, provider, deployType, customName) {
            var msg = qsTr("Model added: ") + name
            var win = ApplicationWindow.window
            if (win && win.toast) win.toast(msg)
        }
    }
}
