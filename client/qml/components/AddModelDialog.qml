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
    signal saved(string name, string provider, string deployType, string customName)

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
            onSelected: function(index, value) { root.providerIndex = index }
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
                    root.saved(
                        root.modelName,
                        root.providers[root.providerIndex],
                        root.deployTypes[root.deployIndex],
                        root.customName
                    )
                    root.close()
                }
            }
        }
    }

    // reset fields before showing（onAboutToShow 在显示前触发，比 onOpened 更可靠）
    onAboutToShow: {
        root.modelName = ""
        root.customName = ""
        root.providerIndex = 0
        root.deployIndex = 0
        nameField.text = ""
        customField.text = ""
        providerSelect.currentIndex = 0
        deploySelect.currentIndex = 0
    }
}
