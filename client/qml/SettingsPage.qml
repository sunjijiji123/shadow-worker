import QtQuick
import QtQuick.Controls
import QtQuick.Layouts

Page {
    id: root
    title: "设置"

    property var viewModel

    Component.onCompleted: viewModel.load()

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 16
        spacing: 12

        RowLayout {
            Layout.fillWidth: true
            spacing: 8
            Label {
                text: "设置"
                font.bold: true
                font.pixelSize: 20
            }
            Item { Layout.fillWidth: true }
            BusyIndicator {
                running: viewModel ? viewModel.loading : false
                implicitWidth: 24
                implicitHeight: 24
            }
            Button {
                text: "重新加载"
                onClicked: viewModel.load()
            }
            Button {
                text: "保存"
                onClicked: viewModel.save()
            }
        }

        Label {
            visible: viewModel && viewModel.error.length > 0
            text: viewModel ? viewModel.error : ""
            color: "red"
            Layout.fillWidth: true
            wrapMode: Text.Wrap
        }

        ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true

            ColumnLayout {
                width: parent.width
                spacing: 16

                GroupBox {
                    title: "热键"
                    Layout.fillWidth: true
                    GridLayout {
                        anchors.fill: parent
                        columns: 2
                        rowSpacing: 8
                        columnSpacing: 12
                        Label { text: "录音/语音输入" }
                        TextField {
                            text: viewModel ? viewModel.hotkeyRecord : "F9"
                            onEditingFinished: if (viewModel) viewModel.hotkeyRecord = text
                            Layout.fillWidth: true
                        }
                        Label { text: "截图理解" }
                        TextField {
                            text: viewModel ? viewModel.hotkeyScreenshot : ""
                            onEditingFinished: if (viewModel) viewModel.hotkeyScreenshot = text
                            Layout.fillWidth: true
                        }
                        Label { text: "提示词前缀键" }
                        ComboBox {
                            model: ["Ctrl", "Alt", "Win"]
                            currentIndex: viewModel ? model.indexOf(viewModel.hotkeyPromptPrefix) : 0
                            onActivated: (index) => { if (viewModel) viewModel.hotkeyPromptPrefix = model[index]; }
                            Layout.fillWidth: true
                        }
                    }
                }

                GroupBox {
                    title: "ASR 语音"
                    Layout.fillWidth: true
                    ColumnLayout {
                        anchors.fill: parent
                        spacing: 12

                        GridLayout {
                            columns: 2
                            rowSpacing: 8
                            columnSpacing: 12
                            Layout.fillWidth: true
                            Label { text: "模式" }
                            ComboBox {
                                model: ["cloud", "local"]
                                currentIndex: viewModel ? model.indexOf(viewModel.asrMode) : 0
                                onActivated: (index) => { if (viewModel) viewModel.asrMode = model[index]; }
                                Layout.fillWidth: true
                            }
                            Label { text: "当前 Provider" }
                            ComboBox {
                                model: viewModel ? viewModel.asrProviders : []
                                textRole: "key"
                                currentIndex: viewModel ? indexOfValue(viewModel.asrActiveProvider) : -1
                                onActivated: (index) => {
                                    if (viewModel && currentValue !== undefined)
                                        viewModel.setActiveProvider("asr", currentValue)
                                }
                                Layout.fillWidth: true
                            }
                        }

                        Label {
                            text: "本地模型配置"
                            font.bold: true
                        }
                        GridLayout {
                            columns: 2
                            rowSpacing: 8
                            columnSpacing: 12
                            Layout.fillWidth: true
                            Label { text: "模型路径" }
                            TextField {
                                text: viewModel ? viewModel.asrLocalModelPath : ""
                                onEditingFinished: if (viewModel) viewModel.asrLocalModelPath = text
                                Layout.fillWidth: true
                            }
                            Label { text: "模型名称" }
                            TextField {
                                text: viewModel ? viewModel.asrLocalModelName : ""
                                onEditingFinished: if (viewModel) viewModel.asrLocalModelName = text
                                Layout.fillWidth: true
                            }
                            Label { text: "语言" }
                            TextField {
                                text: viewModel ? viewModel.asrLocalLanguage : "zh"
                                onEditingFinished: if (viewModel) viewModel.asrLocalLanguage = text
                                Layout.fillWidth: true
                            }
                        }

                        ProviderEditor {
                            Layout.fillWidth: true
                            category: "asr"
                            viewModel: root.viewModel
                        }
                    }
                }

                GroupBox {
                    title: "VLM 截图理解"
                    Layout.fillWidth: true
                    ColumnLayout {
                        anchors.fill: parent
                        spacing: 12
                        GridLayout {
                            columns: 2
                            rowSpacing: 8
                            columnSpacing: 12
                            Layout.fillWidth: true
                            Label { text: "模式" }
                            ComboBox {
                                model: ["off", "scheduled", "on_demand"]
                                currentIndex: viewModel ? model.indexOf(viewModel.vlmMode) : 0
                                onActivated: (index) => { if (viewModel) viewModel.vlmMode = model[index]; }
                                Layout.fillWidth: true
                            }
                            Label { text: "当前 Provider" }
                            ComboBox {
                                model: viewModel ? viewModel.vlmProviders : []
                                textRole: "key"
                                currentIndex: viewModel ? indexOfValue(viewModel.vlmActiveProvider) : -1
                                onActivated: (index) => {
                                    if (viewModel && currentValue !== undefined)
                                        viewModel.setActiveProvider("vlm", currentValue)
                                }
                                Layout.fillWidth: true
                            }
                            Label { text: "定时间隔(分)" }
                            SpinBox {
                                from: 1; to: 60
                                value: viewModel ? viewModel.vlmInterval : 5
                                onValueModified: if (viewModel) viewModel.vlmInterval = value
                                Layout.fillWidth: true
                            }
                        }
                        ProviderEditor {
                            Layout.fillWidth: true
                            category: "vlm"
                            viewModel: root.viewModel
                        }
                    }
                }

                GroupBox {
                    title: "LLM / 润色"
                    Layout.fillWidth: true
                    ColumnLayout {
                        anchors.fill: parent
                        spacing: 12
                        GridLayout {
                            columns: 2
                            rowSpacing: 8
                            columnSpacing: 12
                            Layout.fillWidth: true
                            Label { text: "启用" }
                            CheckBox {
                                checked: viewModel ? viewModel.llmEnabled : false
                                onClicked: if (viewModel) viewModel.llmEnabled = checked
                            }
                            Label { text: "当前 Provider" }
                            ComboBox {
                                model: viewModel ? viewModel.llmProviders : []
                                textRole: "key"
                                currentIndex: viewModel ? indexOfValue(viewModel.llmActiveProvider) : -1
                                onActivated: (index) => {
                                    if (viewModel && currentValue !== undefined)
                                        viewModel.setActiveProvider("llm", currentValue)
                                }
                                Layout.fillWidth: true
                            }
                            Label { text: "注入模式" }
                            ComboBox {
                                model: ["preview", "auto"]
                                currentIndex: viewModel ? model.indexOf(viewModel.llmInjectMode) : 0
                                onActivated: (index) => { if (viewModel) viewModel.llmInjectMode = model[index]; }
                                Layout.fillWidth: true
                            }
                            Label { text: "润色提示词" }
                            TextField {
                                text: viewModel ? viewModel.llmPrompt : ""
                                onEditingFinished: if (viewModel) viewModel.llmPrompt = text
                                Layout.fillWidth: true
                            }
                        }
                        ProviderEditor {
                            Layout.fillWidth: true
                            category: "llm"
                            viewModel: root.viewModel
                        }
                    }
                }

                GroupBox {
                    title: "行为采集"
                    Layout.fillWidth: true
                    GridLayout {
                        anchors.fill: parent
                        columns: 2
                        rowSpacing: 8
                        columnSpacing: 12
                        Label { text: "采样间隔(ms)" }
                        SpinBox {
                            from: 100; to: 2000; stepSize: 50
                            value: viewModel ? viewModel.movementSampleMs : 300
                            onValueModified: if (viewModel) viewModel.movementSampleMs = value
                            Layout.fillWidth: true
                        }
                        Label { text: "空闲超时(s)" }
                        SpinBox {
                            from: 1; to: 300
                            value: viewModel ? viewModel.movementIdleS : 10
                            onValueModified: if (viewModel) viewModel.movementIdleS = value
                            Layout.fillWidth: true
                        }
                        Label { text: "精度" }
                        ComboBox {
                            model: ["low", "medium", "high"]
                            currentIndex: viewModel ? model.indexOf(viewModel.movementPrecision) : 1
                            onActivated: (index) => { if (viewModel) viewModel.movementPrecision = model[index]; }
                            Layout.fillWidth: true
                        }
                    }
                }

                GroupBox {
                    title: "系统"
                    Layout.fillWidth: true
                    RowLayout {
                        anchors.fill: parent
                        spacing: 12
                        Label { text: "开机自启" }
                        CheckBox {
                            checked: autostartManager ? autostartManager.enabled : false
                            onClicked: {
                                if (autostartManager)
                                    autostartManager.enabled = checked;
                            }
                        }
                    }
                }
            }
        }
    }

    component ProviderEditor: ColumnLayout {
        property string category
        property var viewModel

        Label {
            text: "Provider 列表"
            font.bold: true
        }

        Repeater {
            model: viewModel ? (category === "asr" ? viewModel.asrProviders :
                                category === "vlm" ? viewModel.vlmProviders :
                                viewModel.llmProviders) : []

            Frame {
                Layout.fillWidth: true
                property var providerData: modelData
                ColumnLayout {
                    anchors.fill: parent
                    spacing: 8

                    GridLayout {
                        columns: 2
                        rowSpacing: 6
                        columnSpacing: 10
                        Layout.fillWidth: true

                        Label { text: "标识"; font.pixelSize: 12 }
                        Label { text: providerData.key; font.bold: true; Layout.fillWidth: true }

                        Label { text: "名称"; font.pixelSize: 12 }
                        TextField {
                            text: providerData.name
                            onEditingFinished: viewModel.updateProvider(category, providerData.key, { name: text })
                            Layout.fillWidth: true
                        }

                        Label { text: "Base URL"; font.pixelSize: 12 }
                        TextField {
                            text: providerData.baseUrl
                            onEditingFinished: viewModel.updateProvider(category, providerData.key, { baseUrl: text })
                            Layout.fillWidth: true
                        }

                        Label { text: "Model"; font.pixelSize: 12 }
                        TextField {
                            text: providerData.model
                            onEditingFinished: viewModel.updateProvider(category, providerData.key, { model: text })
                            Layout.fillWidth: true
                        }

                        Label { text: "API Key"; font.pixelSize: 12 }
                        TextField {
                            text: providerData.apiKey
                            echoMode: TextInput.Password
                            onEditingFinished: viewModel.updateProvider(category, providerData.key, { apiKey: text })
                            Layout.fillWidth: true
                        }

                        Label { text: "Auth Type"; font.pixelSize: 12 }
                        ComboBox {
                            model: ["bearer", "api-key"]
                            currentIndex: model.indexOf(providerData.authType)
                            onActivated: (index) => viewModel.updateProvider(category, providerData.key, { authType: model[index] })
                            Layout.fillWidth: true
                        }

                        Label { text: "API Format"; font.pixelSize: 12 }
                        ComboBox {
                            model: ["openai", "anthropic", "ollama"]
                            currentIndex: model.indexOf(providerData.apiFormat)
                            onActivated: (index) => viewModel.updateProvider(category, providerData.key, { apiFormat: model[index] })
                            Layout.fillWidth: true
                        }

                        Label { text: "Num Ctx (Ollama)"; font.pixelSize: 12 }
                        SpinBox {
                            from: 0; to: 65536; stepSize: 1024
                            value: providerData.numCtx
                            onValueModified: viewModel.updateProvider(category, providerData.key, { numCtx: value })
                            Layout.fillWidth: true
                        }
                    }

                    Button {
                        text: "删除"
                        onClicked: viewModel.removeProvider(category, providerData.key)
                    }
                }
            }
        }

        RowLayout {
            spacing: 8
            TextField {
                id: newKeyField
                placeholderText: "新 provider key"
                Layout.fillWidth: true
            }
            Button {
                text: "添加"
                onClicked: {
                    if (newKeyField.text !== "") {
                        viewModel.addProvider(category, newKeyField.text)
                        newKeyField.text = ""
                    }
                }
            }
        }
    }
}
