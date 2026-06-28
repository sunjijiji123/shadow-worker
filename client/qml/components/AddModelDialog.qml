// AddModelDialog.qml - add model dialog (HTML #add-model-modal).
// Fields: display name / provider select / deploy type select / custom name (conditional).
// Uses QtQuick.Controls Dialog (Overlay layer).

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
    width: 420
    padding: 20
    topPadding: 20
    bottomPadding: 20
    leftPadding: 20
    rightPadding: 20

    // signals
    signal saved(string name, string provider, string deployType, string customName, var fields)

    // local state
    property string modelName: ""
    property int providerIndex: 0
    property int deployIndex: 0
    property string customName: ""

    // 三种模型类型各有不同的预设 provider 列表。
    // SettingsPage 在 open 前设 targetCategory（"asr"|"vlm"|"llm"）。
    // 用绑定表达式（而非 onTargetCategoryChanged）：前者依赖值变化才触发，
    // 如果连续在同一种 tab 操作（targetCategory 值未变），providers 不会刷新。
    // 绑定表达式每次求值都正确，无此问题。
    property string targetCategory: "asr"
    readonly property var asrProviders: ["Xiaomi MIMO", "BigModel GLM", qsTr("Custom")]
    readonly property var llmProviders: ["DeepSeek", "BigModel GLM", "OpenAI", qsTr("Custom")]
    readonly property var vlmProviders: ["BigModel GLM", "Ollama", qsTr("Custom")]
    readonly property var providers: {
        if (targetCategory === "llm") return llmProviders
        if (targetCategory === "vlm") return vlmProviders
        return asrProviders
    }
    property var deployTypes: [qsTr("Cloud API"), qsTr("Local Model")]

    // 预设配置表：选预设厂商时返回它的 model/baseURL/authType 等字段，
    // 用户只需填 API Key。Custom 返回空 map（全留空自己填）。
    // 返回 {model, baseUrl, authType, apiFormat, stream}。
    function presetFields(providerName) {
        if (providerName === qsTr("Custom")) return {}
        // ASR 预设
        if (targetCategory === "asr") {
            if (providerName === "Xiaomi MIMO") return {
                model: "mimo-v2.5-asr",
                baseUrl: "https://api.xiaomimimo.com/v1/chat/completions",
                authType: "bearer", apiFormat: "openai", stream: true
            }
            if (providerName === "BigModel GLM") return {
                model: "glm-asr-2512",
                baseUrl: "https://open.bigmodel.cn/api/paas/v4/audio/transcriptions",
                authType: "bearer", apiFormat: "openai", stream: false
            }
        }
        // LLM 预设
        if (targetCategory === "llm") {
            if (providerName === "DeepSeek") return {
                model: "deepseek-chat",
                baseUrl: "https://api.deepseek.com/v1",
                authType: "bearer", apiFormat: "openai"
            }
            if (providerName === "BigModel GLM") return {
                model: "glm-4-plus",
                baseUrl: "https://open.bigmodel.cn/api/paas/v4",
                authType: "bearer", apiFormat: "openai"
            }
            if (providerName === "OpenAI") return {
                model: "gpt-4o",
                baseUrl: "https://api.openai.com/v1",
                authType: "bearer", apiFormat: "openai"
            }
        }
        // VLM 预设
        if (targetCategory === "vlm") {
            if (providerName === "BigModel GLM") return {
                // glm-4.6v-flash 是 GLM-4.6V 的免费版，OpenAI 兼容视觉接口，
                // 适合高频定时截图摘要。glm-4v-plus 仍可用，按需替换。
                model: "glm-4.6v-flash",
                baseUrl: "https://open.bigmodel.cn/api/paas/v4",
                authType: "bearer", apiFormat: "openai"
            }
            if (providerName === "Ollama") return {
                model: "llava",
                baseUrl: "http://127.0.0.1:11434",
                authType: "", apiFormat: "openai"
            }
        }
        return {}
    }

    background: Rectangle {
        color: Theme.bg3
        border.color: Theme.rule
        border.width: 1
        radius: 12
    }

    contentItem: ColumnLayout {
        spacing: 12

        Text {
            text: qsTr("Add Model")
            color: Theme.ink
            font.pixelSize: 16
            font.weight: Font.DemiBold
            Layout.fillWidth: true
        }

        // model display name
        TextField {
            id: nameField
            Layout.fillWidth: true
            label: qsTr("Display Name")
            placeholder: qsTr("e.g. Xiaomi ASR Test")
            onTextEdited: root.modelName = newText
        }

        // provider select
        SelectBox {
            id: providerSelect
            Layout.fillWidth: true
            label: qsTr("Provider")
            options: root.providers
            // currentIndex 由 onOpened 显式设置，不用绑定
            onSelected: function(index, value) {
                root.providerIndex = index
                // 选预设厂商时自动填显示名（Custom 不填，让用户输入）
                if (value !== qsTr("Custom")) {
                    nameField.text = value
                    root.modelName = value
                }
            }
        }

        // deploy type select
        SelectBox {
            id: deploySelect
            Layout.fillWidth: true
            label: qsTr("Deploy Type")
            options: root.deployTypes
            // currentIndex 由 onOpened 显式设置，不用绑定
            onSelected: function(index, value) { root.deployIndex = index }
        }

        // custom provider name (visible only when "Custom" selected)
        TextField {
            id: customField
            Layout.fillWidth: true
            visible: root.providers[root.providerIndex] === qsTr("Custom")
            label: qsTr("Custom Provider Name")
            placeholder: qsTr("e.g. My Private Service")
            onTextEdited: root.customName = newText
        }

        // footer: cancel + save
        RowLayout {
            Layout.fillWidth: true
            Layout.topMargin: 8
            spacing: 10

            Item { Layout.fillWidth: true }

            Button {
                text: qsTr("Cancel")
                kind: "ghost"
                onClicked: root.close()
            }
            Button {
                text: qsTr("Save")
                kind: "primary"
                onClicked: {
                    var providerName = root.providers[root.providerIndex]
                    // 预设厂商自动填充字段；Custom 传空 map（全留空）
                    var fields = root.presetFields(providerName)
                    root.saved(
                        root.modelName,
                        providerName,
                        root.deployTypes[root.deployIndex],
                        root.customName,
                        fields
                    )
                    root.close()
                }
            }
        }
    }

    // reset fields when fully opened (not AboutToShow). onOpened 在弹窗完全显示
    // 后触发，此时所有子控件已就绪，可同步清空，不需要 Qt.callLater。
    //
    // 【坑 #2 变体】TextField 的 text 属性有 `text: root.text` 绑定，onTextEdited
    // 不回写 root.text。若在 AboutToShow + callLater 清空，存在时序竞态：
    // providerSelect 默认 index=0 会在某些路径触发预设填充，把厂商名写进
    // nameField，而 callLater 清空可能早于或晚于该填充，导致残留。
    // 改用 onOpened 同步清空所有字段 + 重置 providerSelect，确保每次都是全新状态。
    onOpened: {
        // 清空本地暂存
        root.modelName = ""
        root.customName = ""
        root.providerIndex = 0
        root.deployIndex = 0
        // 同步清空子控件（onOpened 时子控件已就绪）
        nameField.text = ""
        customField.text = ""
        providerSelect.currentIndex = 0
        deploySelect.currentIndex = 0
    }
}
