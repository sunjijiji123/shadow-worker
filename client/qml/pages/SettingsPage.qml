// SettingsPage.qml - settings page skeleton (v2).
// 6 tabs (Voice/Capture/Vision/Polish/Personal/Tools) + bottom unified save bar.
// M2: tab framework + per-tab placeholders.
// M3/M4 TODO: model chips + add dialog + form fields.

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import ShadowWorker

Item {
    id: root

    property var viewModel: null
    property var whitelistViewModel: null
    property var windowPicker: null

    property string activeTab: "voice"   // voice/apps/vision/polish/personal/tools

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 20
        spacing: 16

        Text {
            text: qsTr("Settings")
            color: Theme.ink
            font.pixelSize: Theme.fontTitle
            font.weight: Font.DemiBold
        }

        // ---- tab-strip (6 tabs) ----
        Row {
            Layout.fillWidth: true
            spacing: 8

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

                    Item {
                        width: tabLbl.implicitWidth + 8
                        height: 32
                        Text {
                            id: tabLbl
                            anchors.centerIn: parent
                            text: modelData.label
                            color: activeTab === modelData.tab ? Theme.accent : Theme.muted
                            font.pixelSize: Theme.fontBody
                        }
                        MouseArea {
                            anchors.fill: parent
                            cursorShape: Qt.PointingHandCursor
                            onClicked: activeTab = modelData.tab
                        }
                    }
                    Rectangle {
                        width: tabLbl.implicitWidth + 8
                        height: 2
                        color: activeTab === modelData.tab ? Theme.accent : "transparent"
                    }
                }
            }
        }

        // ---- tab content area (M3/M4 fills forms) ----
        ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true

            ColumnLayout {
                width: parent ? parent.width : 600
                spacing: 16

                Card {
                    Layout.fillWidth: true
                    visible: activeTab === "voice"
                    title: qsTr("Record Hotkey")
                    description: qsTr("M3: master toggle + hold/press radio + modifier/key")
                    PlaceholderLabel { text: qsTr("Pending M3") }
                }
                Card {
                    Layout.fillWidth: true
                    visible: activeTab === "voice"
                    title: qsTr("ASR Model Service")
                    description: qsTr("M3: model chips + add dialog + cloud/local form + test")
                    PlaceholderLabel { text: qsTr("Pending M3") }
                }

                Card {
                    Layout.fillWidth: true
                    visible: activeTab === "apps"
                    title: qsTr("Tracked Apps (Whitelist)")
                    description: qsTr("M3: card list + category chips + add (window picker)")
                    PlaceholderLabel { text: qsTr("Pending M3 (VM whitelistViewModel wired)") }
                }
                Card {
                    Layout.fillWidth: true
                    visible: activeTab === "apps"
                    title: qsTr("Capture Rules")
                    description: qsTr("M3: pause-on-lockscreen toggle + idle timeout")
                    PlaceholderLabel { text: qsTr("Pending M3") }
                }

                Card {
                    Layout.fillWidth: true
                    visible: activeTab === "vision"
                    title: qsTr("VLM / Model Service / Range / Params")
                    description: qsTr("M3: four cards")
                    PlaceholderLabel { text: qsTr("Pending M3") }
                }

                Card {
                    Layout.fillWidth: true
                    visible: activeTab === "polish"
                    title: qsTr("Auto Polish / LLM Model / Polish Prompt")
                    description: qsTr("M3: three cards")
                    PlaceholderLabel { text: qsTr("Pending M3") }
                }

                Card {
                    Layout.fillWidth: true
                    visible: activeTab === "personal"
                    title: qsTr("Quick Inject / Personal Prompt List")
                    description: qsTr("M3: toggle + prefix key + list add/remove")
                    PlaceholderLabel { text: qsTr("Pending M3") }
                }

                Card {
                    Layout.fillWidth: true
                    visible: activeTab === "tools"
                    title: qsTr("Screenshot / Data Management")
                    description: qsTr("M3: two cards")
                    PlaceholderLabel { text: qsTr("Pending M3") }
                }
            }
        }
    }

    // bottom unified save bar (M3 wires SaveConfig)
    SaveBar {
        onSaveRequested: {
            // M3: viewModel.saveConfig()
            // temp: walk up to ApplicationWindow and call global toast
            var w = root
            while (w && !w.toast) { w = w.parent }
            if (w && w.toast) w.toast(qsTr("Settings saved"))
        }
    }
}
