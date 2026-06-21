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

    // reset fields before showing. onAboutToShow 在显示前触发，
    // 但部分子控件此刻可能尚未完全就绪（AGENTS.md 坑 #4：Component.onCompleted
    // 里同步改子组件会触发 ASSERT）。用 Qt.callLater 延迟到下一事件循环，
    // 确保清空操作在弹窗与子控件都 ready 后执行，彻底清掉上次残留。
    onAboutToShow: {
        // 先同步清掉本地暂存（这些是普通 property，无副作用，立即生效）
        root.modelName = ""
        root.customName = ""
        root.providerIndex = 0
        root.deployIndex = 0
        // 子控件文本/index 的清空延迟到下一帧，避免在组件未就绪阶段赋值被吞
        Qt.callLater(function() {
            if (nameField) nameField.text = ""
            if (customField) customField.text = ""
            if (providerSelect) providerSelect.currentIndex = 0
            if (deploySelect) deploySelect.currentIndex = 0
        })
    }
}
