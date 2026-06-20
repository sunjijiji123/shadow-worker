// SettingsPage.qml - settings page (v2).
// 6 tabs all implemented: Voice / Vision / Polish / Personal / Apps / Tools.
// Matches ui-wireframe-v2.html settings section.

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import QtQuick.Dialogs
import ShadowWorker

Item {
    id: root

    property var viewModel: null
    property var whitelistViewModel: null
    property var windowPicker: null

    property string activeTab: "voice"

    // ---- Voice tab state (bound to viewModel) ----
    property bool recordEnabled: true
    property string recordMode: "hold"   // hold | press
    property string asrMode: "cloud"     // cloud | local (整体 ASR 模式，由 active provider type 推导)
    property string asrActiveModel: ""   // 当前选中 provider 的 key
    property string asrModelType: "cloud"  // cloud | local (active provider 的类型)
    property string asrLocalModelPath: ""
    property string asrLocalModelName: ""
    property string asrLocalLanguage: "zh"
    property int micTestLevel: 0

    // 云端字段暂存（直接双向绑定 TextField，Save 时一次性写入 viewModel）
    property string cloudName: ""
    property string cloudBaseUrl: ""
    property string cloudModel: ""
    property string cloudApiKey: ""
    property string cloudLang: "zh"
    property string cloudApiFmt: "openai"
    property string cloudAuth: "bearer"

    // ---- record hotkey: modifier + key, parsed from settingsVm.hotkeyRecord ----
    // e.g. "Ctrl+Shift+R" -> modifier="Ctrl + Shift" (UI label), key="R"
    property string hotkeyModifier: "Ctrl + Shift"
    property string hotkeyKey: "R"

    // build the globalHotkey shortcut string from modifier + key.
    // modifier UI labels use " + " separators; the registration API uses "+".
    function hotkeyString(mod, key) {
        var m = mod === qsTr("None") ? "" : mod.replace(/ \+ /g, "+")
        return m === "" ? key : (m + "+" + key)
    }
    // (re)register the record hotkey with the OS whenever modifier/key changes.
    // 防抖注册热键：ViewModel 异步加载时多个 changed 信号会密集触发，
    // 用户快速切换 mode/key 也会连续触发。短时间内的多次注册（尤其 hold
    // 模式涉及低级键盘钩子）可能让系统键盘输入卡顿。统一用 150ms 防抖，
    // 只有最后一次调用后 150ms 才真正执行注册。
    function registerRecordHotkey() {
        hotkeyRegTimer.restart()
    }
    function doRegisterRecordHotkey() {
        if (!globalHotkey) return
        if (!recordEnabled) {
            globalHotkey.unregisterAll()
            return
        }
        var sc = hotkeyString(hotkeyModifier, hotkeyKey)
        globalHotkey.unregisterAll()
        if (settingsVm) settingsVm.hotkeyRecord = sc
        // 根据 recordMode 选择注册模式：hold（按住录音）或 press（toggle）
        var mode = recordMode === "hold" ? "hold" : "press"
        globalHotkey.registerShortcut(sc, "record", mode)
    }
    Timer {
        id: hotkeyRegTimer
        interval: 150
        repeat: false
        onTriggered: doRegisterRecordHotkey()
    }
    // parse settingsVm.hotkeyRecord ("Ctrl+Shift+R") into modifier label + key.
    // Called once on completed to seed the UI fields.
    function initHotkeyFromSettings() {
        var sc = settingsVm && settingsVm.hotkeyRecord ? settingsVm.hotkeyRecord : "Ctrl+Shift+R"
        var parts = sc.split("+")
        var key = parts[parts.length - 1]
        var mods = parts.slice(0, parts.length - 1)
        hotkeyKey = key
        if (mods.length === 0) {
            hotkeyModifier = qsTr("None")
        } else {
            hotkeyModifier = mods.join(" + ")
        }
    }
    // ASR model chip list (derived from viewModel.asrProviders on load).
    // Structure: { key, label, type: "cloud"|"local", deletable: true }
    property var asrModels: []

    // ---- Vision tab local state (will bind to viewModel later) ----
    property bool vlmEnabled: true
    property string vlmMode: "scheduled"   // scheduled | ondemand
    property string vlmActiveModel: "xiaomi"
    property string vlmModelType: "cloud"  // cloud | local (from active chip)
    property string captureRange: "active" // screen | active
    // VLM model list (mutable so chips can be removed at runtime)
    property var vlmModels: [
        { key: "xiaomi",   label: "Xiaomi-VLM-1", type: "cloud", deletable: true },
        { key: "deepseek", label: "DeepSeek-VL",  type: "cloud", deletable: true },
        { key: "local",    label: "Local-ollama", type: "local", deletable: true }
    ]

    // ---- Polish tab local state (will bind to viewModel later) ----
    property bool polishEnabled: true
    property string llmActiveModel: "deepseek"
    property string llmModelType: "cloud"  // cloud | local (from active chip)
    // LLM model list (mutable so chips can be removed at runtime)
    property var llmModels: [
        { key: "deepseek", label: "DeepSeek-LLM", type: "cloud", deletable: true },
        { key: "xiaomi",   label: "Xiaomi-LLM-1", type: "cloud", deletable: true },
        { key: "local",    label: "Local-ollama", type: "local", deletable: true }
    ]
    // polish prompt content. Default comes from the backend (default_prompt.txt)
    // via viewModel at runtime; empty here so no Chinese literals live in code.
    property string polishPrompt: ""

    // ---- Personal Prompts tab local state (will bind to viewModel later) ----
    property bool quickInjectEnabled: true
    property string promptPrefixKey: "Ctrl"   // Ctrl | Alt | Win
    // personal prompt list (mutable: add / delete / edit)
    // Note: prompt text content is data; placeholder-only here (no Chinese literals).
    property var personalPrompts: [
        { name: "", key: "1", text: "" },
        { name: "", key: "2", text: "" }
    ]
    property string nextPromptKey: "3"   // next key to assign on add

    // add an empty prompt row, auto-assigning the next available key (0-9, A-Z)
    function addPersonalPrompt() {
        var arr = personalPrompts.slice()
        // find next free single-char key
        var used = {}
        for (var i = 0; i < arr.length; i++) used[arr[i].key] = true
        var candidate = ""
        var chars = "1234567890QWERTYUIOPASDFGHJKLZXCVBNM"
        for (var j = 0; j < chars.length; j++) {
            if (!used[chars[j]]) { candidate = chars[j]; break }
        }
        if (candidate === "") {
            var win = ApplicationWindow.window
            if (win && win.toast) win.toast(qsTr("No free shortcut keys available"), "warning")
            return
        }
        arr.push({ name: "", key: candidate, text: "" })
        personalPrompts = arr
    }

    // delete a prompt by index
    function deletePersonalPrompt(index) {
        var arr = personalPrompts.slice()
        arr.splice(index, 1)
        personalPrompts = arr
    }

    // update a single field on a prompt by index
    function updatePersonalPrompt(index, field, value) {
        var arr = personalPrompts.slice()
        if (index >= 0 && index < arr.length) {
            arr[index][field] = value
            personalPrompts = arr
        }
    }

    // ---- Behavior Capture (apps) tab local state ----
    property bool pauseOnLock: true
    property int idleTimeoutMin: 5
    // tracked app whitelist: {key, name, iconBg, iconText, category}
    // category must be one of: coding / browser / chat / office / other
    property var trackedApps: [
        { key: "cursor",  name: "Cursor",  iconBg: "#3B82F6", iconText: "Cr", category: "coding" },
        { key: "chrome",  name: "Chrome",  iconBg: "#F59E0B", iconText: "Ch", category: "browser" },
        { key: "wechat",  name: "WeChat",  iconBg: "#10B981", iconText: "We", category: "chat" }
    ]

    function setAppCategory(index, category) {
        var arr = trackedApps.slice()
        if (index >= 0 && index < arr.length) {
            arr[index].category = category
            trackedApps = arr
        }
    }
    function removeTrackedApp(index) {
        var arr = trackedApps.slice()
        arr.splice(index, 1)
        trackedApps = arr
    }

    // ---- Quick Tools tab local state ----
    property string screenshotModifier: "Ctrl + Shift"
    property string screenshotKey: "S"
    property string screenshotSavePath: "C:\\Users\\…\\shadow-worker\\screenshots"

    // Remove a model from a list and re-select if the active one was removed.
    // Usage: removeModel("asrModels", "deepseek", "asrActiveModel", "asrModelType")
    function removeModel(listName, key, activeName, typeName) {
        var arr = root[listName]
        var next = []
        for (var i = 0; i < arr.length; i++) {
            if (arr[i].key !== key) next.push(arr[i])
        }
        root[listName] = next
        if (root[activeName] === key && next.length > 0) {
            root[activeName] = next[0].key
            root[typeName] = next[0].type === "local" ? "local" : "cloud"
        }
    }

    // ---- pending model deletion (two-step confirm) ----
    // When the user clicks × on a chip, we don't delete immediately — we stash
    // the target here and open deleteConfirmDialog. Confirmation runs removeModel.
    property string pendingListName: ""
    property string pendingKey: ""
    property string pendingActiveName: ""
    property string pendingTypeName: ""
    property string pendingLabel: ""

    // Called from ModelChipGroup.onChipClosed; opens the confirm dialog.
    function requestDeleteModel(listName, key, activeName, typeName) {
        // find the display label for the dialog message
        var arr = root[listName]
        var label = key
        for (var i = 0; i < arr.length; i++) {
            if (arr[i].key === key) { label = arr[i].label; break }
        }
        root.pendingListName = listName
        root.pendingKey = key
        root.pendingActiveName = activeName
        root.pendingTypeName = typeName
        root.pendingLabel = label
        deleteConfirmDialog.open()
    }

    // Load config from the backend on entry, then sync the local UI props
    // from the viewModel once loaded. Also seed the OS hotkey registration.
    Component.onCompleted: {
        if (viewModel) {
            viewModel.load()
            // sync local props when the async load completes. We watch the
            // notify signals of key properties as a "load done" cue.
            // 用 Qt.callLater 延迟到下一事件循环，避免在 componentComplete
            // 阶段修改子组件属性触发 QQuickItem 的 m_componentComplete 断言。
            viewModel.hotkeyRecordChanged.connect(function() { Qt.callLater(root.syncFromViewModel) })
            viewModel.recordModeChanged.connect(function() { Qt.callLater(root.syncFromViewModel) })
            viewModel.asrActiveProviderChanged.connect(function() { Qt.callLater(root.syncFromViewModel) })
            viewModel.asrProvidersChanged.connect(function() { Qt.callLater(root.syncFromViewModel) })
            viewModel.asrLocalModelPathChanged.connect(function() { Qt.callLater(root.syncFromViewModel) })
            viewModel.asrLocalLanguageChanged.connect(function() { Qt.callLater(root.syncFromViewModel) })
        }
        // seed from whatever the viewModel already holds (defaults until load).
        Qt.callLater(syncFromViewModel)
        // 先用默认值注册一次，确保热键可用。等异步 syncFromViewModel 加载完
        // 真实 recordMode 后，由 Radio 切换或 key/modifier 改变时按需重注册。
        registerRecordHotkey()
    }

    // Test Connection 的响应改由 main.qml 的全局 Connections 处理
    // （这里曾用 Connections 但收不到信号，疑似 import ShadowWorker 的
    //  VoiceClient 类型注册干扰了 context property 的信号查找）


    // 把 viewModel.asrProviders（QVariantList）映射成 chip 结构。
    function providersToChips(providers) {
        var chips = []
        console.log("[ASR] providersToChips: count=" + providers.length)
        for (var i = 0; i < providers.length; i++) {
            var p = providers[i]
            console.log("[ASR]   provider[" + i + "]: key=" + p.key + " name=" + p.name + " type=" + p.type)
            chips.push({
                key: p.key || "",
                label: p.name || p.key || "",
                type: p.type || "cloud",
                deletable: true
            })
        }
        return chips
    }

    // 从 viewModel 拉数据到本地 UI property。
    function syncFromViewModel() {
        if (!viewModel) return
        if (viewModel.recordMode) recordMode = viewModel.recordMode
        initHotkeyFromSettings()

        // ASR providers → chip 列表
        var providers = viewModel.asrProviders || []
        asrModels = providersToChips(providers)

        // active provider
        asrActiveModel = viewModel.asrActiveProvider || ""
        asrMode = viewModel.asrMode || "cloud"

        // 根据 active provider 的 type 推导 asrModelType
        asrModelType = activeProviderType(asrActiveModel)

        // 本地模型字段
        asrLocalModelPath = viewModel.asrLocalModelPath || ""
        asrLocalModelName = viewModel.asrLocalModelName || deriveModelName(asrLocalModelPath)
        asrLocalLanguage = viewModel.asrLocalLanguage || "zh"

        // 灌入云端字段（无 binding，显式赋值）
        updateCloudFields()
    }

    // 查找当前 active provider 的 type。
    function activeProviderType(key) {
        if (!viewModel) return "cloud"
        var providers = viewModel.asrProviders || []
        for (var i = 0; i < providers.length; i++) {
            if (providers[i].key === key) return providers[i].type || "cloud"
        }
        return "cloud"
    }

    // 从模型路径推导显示名（文件名去扩展名）。
    function deriveModelName(path) {
        if (!path) return ""
        var base = path.replace(/\\/g, "/").split("/").pop()
        return base.replace(/\.bin$/i, "")
    }

    // 读取当前 active provider 的某个字段。
    function providerField(key, field) {
        if (!viewModel) return ""
        var providers = viewModel.asrProviders || []
        for (var i = 0; i < providers.length; i++) {
            if (providers[i].key === key) return providers[i][field] || ""
        }
        return ""
    }

    // chip 列表名 → viewModel category。
    function listNameToCategory(listName) {
        if (listName === "asrModels") return "asr"
        if (listName === "vlmModels") return "vlm"
        if (listName === "llmModels") return "llm"
        return ""
    }

    // 语言字符串 ↔ SelectBox index 映射。
    // 选项: 0=zh, 1=en, 2=ja, 3=mixed, 4=auto
    function langToIndex(lang) {
        switch (lang) {
            case "zh": return 0
            case "en": return 1
            case "ja": return 2
            case "mixed": return 3
            case "auto": return 4
            default: return 0
        }
    }
    function indexToLang(idx) {
        return ["zh", "en", "ja", "mixed", "auto"][idx] || "zh"
    }

    // auth 字符串 ↔ SelectBox index。
    // 0=bearer, 1=api-key, 2=""(no auth)
    function authToIndex(auth) {
        switch (auth) {
            case "bearer": return 0
            case "api-key": return 1
            case "": return 2
            default: return 0
        }
    }
    function indexToAuth(idx) {
        return ["bearer", "api-key", ""][idx] || "bearer"
    }

    // 把本地 UI property 推回 viewModel（保存前调用）。
    function pushToViewModel() {
        if (!viewModel) return
        viewModel.recordMode = recordMode
        viewModel.hotkeyRecord = hotkeyString(hotkeyModifier, hotkeyKey)
        viewModel.asrMode = asrModelType
        viewModel.asrActiveProvider = asrActiveModel
        // 保存前：把当前 UI 字段的值 Write-Through 到 viewModel，确保不丢
        flushCloudFields()
        viewModel.asrLocalModelPath = asrLocalModelPath
        viewModel.asrLocalModelName = deriveModelName(asrLocalModelPath)
        viewModel.asrLocalLanguage = asrLocalLanguage
    }

    // 把本地暂存的云端字段 Write-Through 到 viewModel。
    function flushCloudFields() {
        if (!viewModel || !asrActiveModel) { console.log("[flush] skip, model=" + asrActiveModel); return }
        console.log("[flush] baseUrl=" + cloudBaseUrl + " model=" + cloudModel + " key=" + cloudApiKey)
        viewModel.updateProvider("asr", asrActiveModel, {
            name: cloudName, baseUrl: cloudBaseUrl, model: cloudModel,
            apiKey: cloudApiKey, language: cloudLang,
            apiFormat: cloudApiFmt, authType: cloudAuth
        })
    }

    // 从 viewModel 读取当前 provider 数据，灌入本地暂存属性 + UI 控件。
    function updateCloudFields() {
        if (!viewModel || !asrActiveModel) {
            cloudName = ""; cloudBaseUrl = ""; cloudModel = ""; cloudApiKey = ""
            cloudLang = "zh"; cloudApiFmt = "openai"; cloudAuth = "bearer"
        } else {
            cloudName = providerField(asrActiveModel, "name") || asrActiveModel
            cloudBaseUrl = providerField(asrActiveModel, "baseUrl")
            cloudModel = providerField(asrActiveModel, "model")
            cloudApiKey = providerField(asrActiveModel, "apiKey")
            cloudLang = providerField(asrActiveModel, "language") || "zh"
            cloudApiFmt = providerField(asrActiveModel, "apiFormat") || "openai"
            cloudAuth = providerField(asrActiveModel, "authType") || "bearer"
        }
        // 命令式更新 UI（无 binding）
        if (cloudNameField) cloudNameField.text = cloudName
        if (cloudBaseUrlField) cloudBaseUrlField.text = cloudBaseUrl
        if (cloudModelField) cloudModelField.text = cloudModel
        if (cloudKeyField) cloudKeyField.text = cloudApiKey
        if (cloudLangBox) cloudLangBox.currentIndex = langToIndex(cloudLang)
        if (cloudApiFormatBox) cloudApiFormatBox.currentIndex = cloudApiFmt === "anthropic" ? 1 : 0
        if (cloudAuthBox) cloudAuthBox.currentIndex = authToIndex(cloudAuth)
    }

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
                        onToggled: {
                            recordEnabled = checked
                            registerRecordHotkey()
                        }
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
                            onClicked: {
                                recordMode = "hold"
                                registerRecordHotkey()
                            }
                        }
                        Radio {
                            text: qsTr("Press to Record")
                            checked: recordMode === "press"
                            onClicked: {
                                recordMode = "press"
                                registerRecordHotkey()
                            }
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
                            // current index from hotkeyModifier label
                            currentIndex: {
                                var opts = [qsTr("None"), "Ctrl", "Alt", "Win", "Ctrl + Shift"]
                                for (var i = 0; i < opts.length; i++) {
                                    if (opts[i] === hotkeyModifier) return i
                                }
                                return 4
                            }
                            onSelected: function(index, value) {
                                hotkeyModifier = value
                                registerRecordHotkey()
                            }
                        }

                        TextField {
                            Layout.fillWidth: true
                            label: qsTr("Key")
                            text: hotkeyKey
                            captureMode: true
                            placeholder: qsTr("Press a key...")
                            // keyPressed fires when a key is captured (not on
                            // manual typing — captureMode makes the field read-only)
                            onKeyPressed: function(keyName) {
                                hotkeyKey = keyName
                                registerRecordHotkey()
                            }
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
                        chips: asrModels
                        onChipClicked: function(key) {
                            asrActiveModel = key
                            asrModelType = activeProviderType(key)
                            updateCloudFields()
                        }
                        onChipClosed: function(key) {
                            requestDeleteModel("asrModels", key, "asrActiveModel", "asrModelType")
                        }
                        onCloseBlocked: {
                            var win = ApplicationWindow.window
                            if (win && win.toast) win.toast(qsTr("At least one model must be kept"), "warning")
                        }
                        onAddClicked: {
                            addModelDialog.targetCategory = "asr"
                            addModelDialog.open()
                        }
                    }

                    Text {
                        text: qsTr("Switch models to view or edit config. Add multiple for comparison.")
                        color: Theme.muted
                        font.pixelSize: 13
                        wrapMode: Text.WordWrap
                        Layout.fillWidth: true
                    }

                    // ---- Cloud fields (visible when active provider is cloud) ----
                    GridLayout {
                        id: cloudGrid
                        Layout.fillWidth: true
                        visible: asrModelType === "cloud"
                        columns: 2
                        rowSpacing: 12
                        columnSpacing: 16

                        TextField {
                            id: cloudNameField
                            label: qsTr("Display Name")
                            onTextEdited: function(newText) { cloudName = newText }
                            Layout.fillWidth: true
                        }
                        TextField {
                            id: cloudBaseUrlField
                            label: qsTr("Base URL")
                            onTextEdited: function(newText) {
                                cloudBaseUrl = newText
                                console.log("[cloudBaseUrl]=" + cloudBaseUrl)
                            }
                            Layout.fillWidth: true
                        }
                        TextField {
                            id: cloudModelField
                            label: qsTr("Model")
                            onTextEdited: function(newText) { cloudModel = newText }
                            Layout.fillWidth: true
                        }
                        SelectBox {
                            id: cloudLangBox
                            label: qsTr("Language")
                            options: [qsTr("Chinese (zh)"), qsTr("English (en)"), qsTr("Japanese (ja)"), qsTr("Zh+En mixed"), qsTr("Auto-detect")]
                            onSelected: function(index, value) { cloudLang = indexToLang(index) }
                            Layout.fillWidth: true
                        }
                        SelectBox {
                            id: cloudApiFormatBox
                            label: qsTr("API Format")
                            options: ["OpenAI", "Anthropic messages"]
                            onSelected: function(index, value) { cloudApiFmt = index === 1 ? "anthropic" : "openai" }
                            Layout.fillWidth: true
                        }
                        SelectBox {
                            id: cloudAuthBox
                            label: qsTr("Auth Method")
                            options: ["Bearer", "api-key header", qsTr("No auth")]
                            onSelected: function(index, value) { cloudAuth = indexToAuth(index) }
                            Layout.fillWidth: true
                        }
                        // API Key (span full width)
                        TextField {
                            id: cloudKeyField
                            label: qsTr("API Key")
                            isPassword: true
                            onTextEdited: function(newText) { cloudApiKey = newText }
                            Layout.columnSpan: 2
                            Layout.fillWidth: true
                        }
                    }

                    // ---- Local fields (visible when active provider is local) ----
                    ColumnLayout {
                        Layout.fillWidth: true
                        visible: asrModelType === "local"
                        spacing: 12

                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 8

                            TextField {
                                id: localModelPathField
                                Layout.fillWidth: true
                                label: qsTr("Model Path")
                                text: asrLocalModelPath
                                onTextEdited: {
                                    asrLocalModelPath = text
                                    asrLocalModelName = deriveModelName(text)
                                }
                            }
                            Button {
                                text: qsTr("Browse...")
                                kind: "ghost"
                                Layout.alignment: Qt.AlignBottom
                                onClicked: modelFileDialog.open()
                            }
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 16

                            TextField {
                                Layout.fillWidth: true
                                label: qsTr("Model Name (auto from path)")
                                text: asrLocalModelName
                                readOnly: true
                            }
                            SelectBox {
                                Layout.fillWidth: true
                                label: qsTr("Language")
                                options: [qsTr("Chinese (zh)"), qsTr("English (en)"), qsTr("Japanese (ja)"), qsTr("Zh+En mixed"), qsTr("Auto-detect")]
                                currentIndex: langToIndex(asrLocalLanguage)
                                onSelected: function(index, value) {
                                    asrLocalLanguage = indexToLang(index)
                                }
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

                    // test connection button（紧跟在字段区下方，Save 之前）
                    Button {
                        text: qsTr("Test Connection")
                        kind: "primary"
                        Layout.topMargin: 4
                        onClicked: {
                            if (!voiceClient) {
                                var win0 = ApplicationWindow.window
                                if (win0 && win0.toast) win0.toast(qsTr("voiceClient not available"), "error")
                                return
                            }
                            if (asrModelType === "local") {
                                if (!asrLocalModelPath) {
                                    var win1 = ApplicationWindow.window
                                    if (win1 && win1.toast) win1.toast(qsTr("Model path is empty"), "error")
                                } else {
                                    voiceClient.testConnection("local", {modelPath: asrLocalModelPath})
                                }
                            } else {
                                if (!cloudBaseUrl) {
                                    var win2 = ApplicationWindow.window
                                    if (win2 && win2.toast) win2.toast(qsTr("No base URL filled"), "error")
                                } else {
                                    voiceClient.testConnection("cloud", {
                                        baseUrl: cloudBaseUrl,
                                        model: cloudModel,
                                        apiKey: cloudApiKey,
                                        apiFormat: cloudApiFmt,
                                        authType: cloudAuth,
                                        language: cloudLang
                                    })
                                }
                            }
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
                            id: inputDeviceBox
                            Layout.fillWidth: true
                            Layout.preferredWidth: 2
                            label: qsTr("Input Device")
                            // options come from the real device list; first entry
                            // is always the system default.
                            options: {
                                var opts = [qsTr("Default Microphone")]
                                var devs = audioDeviceManager ? audioDeviceManager.devices : []
                                for (var i = 0; i < devs.length; i++) {
                                    if (!devs[i].isDefault)
                                        opts.push(devs[i].description)
                                }
                                return opts
                            }
                            // index 0 = default; otherwise find the selected device
                            // among the non-default entries (opts[i+1]).
                            currentIndex: {
                                var sel = audioDeviceManager ? audioDeviceManager.selectedDeviceId : ""
                                if (sel === "") return 0
                                var devs = audioDeviceManager ? audioDeviceManager.devices : []
                                var pos = 0
                                for (var i = 0; i < devs.length; i++) {
                                    if (devs[i].isDefault) continue
                                    pos++
                                    if (devs[i].id === sel) return pos
                                }
                                return 0
                            }
                            onSelected: function(index, value) {
                                if (index === 0) {
                                    // default
                                    audioDeviceManager.selectedDeviceId = ""
                                    audioRecorder.inputDeviceId = ""
                                } else {
                                    // index>0 maps to the (index-1)-th non-default device
                                    var devs = audioDeviceManager.devices
                                    var pos = 0
                                    for (var i = 0; i < devs.length; i++) {
                                        if (devs[i].isDefault) continue
                                        pos++
                                        if (pos === index) {
                                            audioDeviceManager.selectedDeviceId = devs[i].id
                                            audioRecorder.inputDeviceId = devs[i].id
                                            return
                                        }
                                    }
                                }
                            }
                        }

                        // mic test button (green pill, HTML .mic-test-btn).
                        // testing state is driven by audioRecorder.recording.
                        MicTestButton {
                            Layout.alignment: Qt.AlignBottom
                            testing: audioRecorder.recording
                            onClicked: {
                                if (audioRecorder.recording) {
                                    audioRecorder.stopRecording()
                                } else {
                                    // test mode: capture just to drive the volume bar
                                    audioRecorder.startRecording(true)
                                }
                            }
                        }
                    }

                    // volume bar: live RMS level from the recorder (0..100)
                    VolumeBar {
                        Layout.fillWidth: true
                        level: audioRecorder.level
                    }
                }
            }

            // ================================================================
            // VISION TAB (4 cards, matches HTML .section[data-tab="vision"])
            // ================================================================

            // ---- Card 1: VLM Screen Understanding ----
            // HTML: .vlm-master-card
            Card {
                Layout.fillWidth: true
                visible: activeTab === "vision"
                title: qsTr("VLM Screen Understanding")
                description: qsTr("When enabled, VLM is called on a schedule or by hotkey to understand the current screen and write to the timeline.")
                headerExtra: [
                    Toggle {
                        checked: vlmEnabled
                        onToggled: vlmEnabled = checked
                    }
                ]

                // separator + mode area (visible only when master toggle is on)
                Rectangle { Layout.fillWidth: true; height: 1; color: Theme.rule; visible: vlmEnabled }

                ColumnLayout {
                    Layout.fillWidth: true
                    visible: vlmEnabled
                    spacing: 16

                    // radio group: scheduled / ondemand
                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 10

                        Radio {
                            text: qsTr("Scheduled Screenshot")
                            checked: vlmMode === "scheduled"
                            onClicked: vlmMode = "scheduled"
                        }
                        Radio {
                            text: qsTr("On-Demand Screenshot")
                            checked: vlmMode === "ondemand"
                            onClicked: vlmMode = "ondemand"
                        }
                    }

                    // scheduled config: interval
                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 16
                        visible: vlmMode === "scheduled"

                        TextField {
                            Layout.fillWidth: true
                            label: qsTr("Interval (min)")
                            text: "5"
                        }
                    }

                    // ondemand config: modifier + key
                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 16
                        visible: vlmMode === "ondemand"

                        // modifier (HTML flex:0.6 — wider) + key
                        SelectBox {
                            label: qsTr("Modifier")
                            options: [qsTr("None"), "Ctrl", "Alt", "Win", "Ctrl + Shift"]
                            currentIndex: 4
                            Layout.fillWidth: true
                        }
                        TextField {
                            label: qsTr("Key")
                            text: "V"
                            Layout.fillWidth: true
                        }
                    }
                }
            }

            // ---- Card 2: VLM Model Service ----
            // HTML: .vlm-service-card (same shape as ASR card)
            Card {
                Layout.fillWidth: true
                visible: activeTab === "vision"
                title: qsTr("VLM Model Service")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 16

                    // model chips
                    ModelChipGroup {
                        Layout.fillWidth: true
                        activeKey: vlmActiveModel
                        chips: vlmModels
                        onChipClicked: function(key) {
                            vlmActiveModel = key
                            if (key === "local") vlmModelType = "local"
                            else vlmModelType = "cloud"
                        }
                        onChipClosed: function(key) {
                            requestDeleteModel("vlmModels", key, "vlmActiveModel", "vlmModelType")
                        }
                        onCloseBlocked: {
                            var win = ApplicationWindow.window
                            if (win && win.toast) win.toast(qsTr("At least one model must be kept"), "warning")
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

                    // ---- Cloud fields (visible when vlmModelType === "cloud") ----
                    GridLayout {
                        Layout.fillWidth: true
                        visible: vlmModelType === "cloud"
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
                            text: "https://api.xiaomi.com/v1/vlm"
                            Layout.fillWidth: true
                        }
                        TextField {
                            label: qsTr("Model")
                            text: "mimo-vl"
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

                    // ---- Local fields (visible when vlmModelType === "local") ----
                    ColumnLayout {
                        Layout.fillWidth: true
                        visible: vlmModelType === "local"
                        spacing: 12

                        TextField {
                            Layout.fillWidth: true
                            label: qsTr("Ollama Server URL")
                            text: "http://localhost:11434"
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 16

                            TextField {
                                Layout.fillWidth: true
                                label: qsTr("Model")
                                text: "llava"
                            }
                            SelectBox {
                                Layout.fillWidth: true
                                label: qsTr("Auth Method")
                                options: ["Bearer", qsTr("No auth")]
                                currentIndex: 1
                            }
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
                            text: qsTr("142 ms latency")
                            color: Theme.muted
                            font.pixelSize: 12
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                }
            }

            // ---- Card 3: Capture Range ----
            // HTML: 画面采集范围 (radio + accent hint)
            Card {
                Layout.fillWidth: true
                visible: activeTab === "vision"
                title: qsTr("Capture Range")
                description: qsTr("Screen area captured during VLM screenshots and behavior capture.")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 12

                    Radio {
                        text: qsTr("Entire Screen (all monitors)")
                        checked: captureRange === "screen"
                        onClicked: captureRange = "screen"
                    }
                    Radio {
                        text: qsTr("Active Window Only")
                        checked: captureRange === "active"
                        onClicked: captureRange = "active"
                    }

                    Text {
                        text: qsTr("Tip: in active-window mode, VLM only captures app windows listed in the Behavior Capture whitelist.")
                        color: Theme.accent
                        font.pixelSize: 13
                        wrapMode: Text.WordWrap
                        Layout.fillWidth: true
                    }
                }
            }

            // ---- Card 4: Capture Parameters ----
            // HTML: 采集参数 (3-field form row)
            Card {
                Layout.fillWidth: true
                visible: activeTab === "vision"
                title: qsTr("Capture Parameters")

                RowLayout {
                    Layout.fillWidth: true
                    spacing: 16

                    TextField {
                        Layout.fillWidth: true
                        label: qsTr("Sample Interval (ms)")
                        text: "300"
                    }
                    TextField {
                        Layout.fillWidth: true
                        label: qsTr("Idle Timeout (s)")
                        text: "10"
                    }
                    SelectBox {
                        Layout.fillWidth: true
                        label: qsTr("Precision")
                        options: ["low", "medium", "high"]
                        currentIndex: 1
                    }
                }
            }

            // ================================================================
            // BEHAVIOR CAPTURE (apps) TAB (2 cards, matches HTML .section[data-tab="apps"])
            // ================================================================

            // ---- Card 1: Tracked Apps (whitelist) ----
            // HTML: title + "Scan Apps" / "+ Add" ghost buttons (same row),
            //       desc, then a list of .whitelist-app-row items.
            Card {
                Layout.fillWidth: true
                visible: activeTab === "apps"
                title: qsTr("Tracked Apps")
                description: qsTr("Only apps listed here are recorded to the timeline; in active-window mode, VLM also captures only these apps' windows. Adding enters a window-picker overlay, identified by process path, saved to the Go-managed config.yaml.")
                headerExtra: [
                    Button {
                        text: qsTr("Scan Apps")
                        kind: "ghost"
                        small: true
                        onClicked: {
                            var win = ApplicationWindow.window
                            if (win && win.toast) win.toast(qsTr("Scanning installed apps..."))
                        }
                    },
                    Button {
                        text: qsTr("+ Add")
                        kind: "ghost"
                        small: true
                        onClicked: {
                            var win = ApplicationWindow.window
                            if (win && win.toast) win.toast(qsTr("Window picker not connected yet"))
                        }
                    }
                ]

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 10

                    Repeater {
                        model: trackedApps

                        delegate: Rectangle {
                            id: appRow
                            required property var modelData
                            required property int index
                            // stash the row's app + index before the inner Repeater
                            // overwrites the `modelData`/`index` names.
                            property var app: modelData
                            property int appIndex: index

                            Layout.fillWidth: true
                            // .whitelist-app-row: bg, rule border, radius 10, padding 12
                            color: Theme.bg
                            border.color: Theme.rule
                            border.width: 1
                            radius: 10
                            height: row.implicitHeight + 24

                            RowLayout {
                                id: row
                                anchors.fill: parent
                                anchors.margins: 12
                                spacing: 12

                                // .app-icon: 40x40, radius 8, colored bg, 14px bold white initials
                                Rectangle {
                                    width: 40
                                    height: 40
                                    radius: 8
                                    color: appRow.app.iconBg
                                    Layout.alignment: Qt.AlignVCenter

                                    Text {
                                        anchors.centerIn: parent
                                        text: appRow.app.iconText
                                        color: "#FFFFFF"
                                        font.pixelSize: 14
                                        font.weight: Font.Bold
                                    }
                                }

                                // .app-meta (flex): name + category chip group
                                ColumnLayout {
                                    Layout.fillWidth: true
                                    spacing: 6

                                    Text {
                                        text: appRow.app.name
                                        color: Theme.ink
                                        font.pixelSize: 14
                                        font.weight: Font.DemiBold
                                    }

                                    // category chip group (single-select radio-like)
                                    Row {
                                        spacing: 6
                                        Layout.fillWidth: true

                                        Repeater {
                                            model: ["coding", "browser", "chat", "office", "other"]

                                            delegate: Chip {
                                                id: catChip
                                                required property string modelData
                                                property bool isActive: appRow.app.category === modelData
                                                // .whitelist-app-row .chip: 11px, padding 2x6
                                                text: modelData
                                                checked: isActive
                                                implicitHeight: 22
                                                onClicked: setAppCategory(appRow.appIndex, modelData)
                                            }
                                        }
                                    }
                                }

                                // .remove ×
                                Text {
                                    text: "\u00D7"
                                    color: removeAppMa.containsMouse ? Theme.danger : Theme.muted
                                    font.pixelSize: 16
                                    Layout.alignment: Qt.AlignVCenter
                                    MouseArea {
                                        id: removeAppMa
                                        anchors.fill: parent
                                        cursorShape: Qt.PointingHandCursor
                                        hoverEnabled: true
                                        onClicked: removeTrackedApp(appRow.appIndex)
                                    }
                                }
                            }
                        }
                    }
                }
            }

            // ---- Card 2: Capture Rules ----
            // HTML: .quick-switch rows.
            //   Row 1: pause-on-lock toggle.
            //   Row 2: idle timeout (number input + "min" label) — NOT a toggle.
            Card {
                Layout.fillWidth: true
                visible: activeTab === "apps"
                title: qsTr("Capture Rules")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 0

                    // Row 1: pause on lock screen
                    Rectangle {
                        Layout.fillWidth: true
                        height: 56
                        color: "transparent"

                        Rectangle {
                            anchors.bottom: parent.bottom
                            anchors.left: parent.left
                            anchors.right: parent.right
                            height: 1
                            color: Theme.rule
                        }

                        RowLayout {
                            anchors.fill: parent
                            spacing: 12

                            ColumnLayout {
                                spacing: 2
                                Text {
                                    text: qsTr("Pause capture when locked")
                                    color: Theme.ink
                                    font.pixelSize: 13
                                }
                                Text {
                                    text: qsTr("Auto-stop recording on lock, resume on unlock")
                                    color: Theme.muted
                                    font.pixelSize: 12
                                }
                            }
                            Item { Layout.fillWidth: true }
                            Toggle {
                                checked: pauseOnLock
                                onToggled: pauseOnLock = checked
                            }
                        }
                    }

                    // Row 2: idle timeout (number + unit) — last row, no divider
                    Rectangle {
                        Layout.fillWidth: true
                        height: 56
                        color: "transparent"

                        RowLayout {
                            anchors.fill: parent
                            spacing: 12

                            ColumnLayout {
                                spacing: 2
                                Text {
                                    text: qsTr("Idle timeout threshold")
                                    color: Theme.ink
                                    font.pixelSize: 13
                                }
                                Text {
                                    text: qsTr("No keyboard/mouse activity beyond this marks you as away")
                                    color: Theme.muted
                                    font.pixelSize: 12
                                }
                            }
                            Item { Layout.fillWidth: true }

                            RowLayout {
                                spacing: 8
                                // narrow centered number input (HTML width:50px)
                                Rectangle {
                                    width: 50
                                    height: 36
                                    color: Theme.bg
                                    border.color: idleInput.activeFocus ? Theme.accent : Theme.rule
                                    border.width: 1
                                    radius: 6

                                    TextInput {
                                        id: idleInput
                                        anchors.centerIn: parent
                                        text: root.idleTimeoutMin
                                        color: Theme.ink
                                        font.pixelSize: 13
                                        horizontalAlignment: TextInput.AlignHCenter
                                        validator: IntValidator { bottom: 1; top: 999 }
                                        onTextChanged: {
                                            var v = parseInt(text)
                                            if (!isNaN(v)) root.idleTimeoutMin = v
                                        }
                                    }
                                }
                                Text {
                                    text: qsTr("min")
                                    color: Theme.muted
                                    font.pixelSize: 13
                                }
                            }
                        }
                    }
                }
            }
            // ================================================================
            // POLISH TAB (3 cards, matches HTML .section[data-tab="polish"])
            // ================================================================

            // ---- Card 1: Auto Polish ----
            // HTML: title + toggle on the same row, desc below.
            Card {
                Layout.fillWidth: true
                visible: activeTab === "polish"
                title: qsTr("Auto Polish")
                description: qsTr("When enabled, ASR results are passed through an LLM for polishing before display/injection.")
                headerExtra: [
                    Toggle {
                        checked: polishEnabled
                        onToggled: polishEnabled = checked
                    }
                ]
            }

            // ---- Card 2: LLM Model Service ----
            // HTML: .llm-service-card (same shape as ASR/VLM cards)
            Card {
                Layout.fillWidth: true
                visible: activeTab === "polish"
                title: qsTr("LLM Model Service")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 16

                    // model chips
                    ModelChipGroup {
                        Layout.fillWidth: true
                        activeKey: llmActiveModel
                        chips: llmModels
                        onChipClicked: function(key) {
                            llmActiveModel = key
                            if (key === "local") llmModelType = "local"
                            else llmModelType = "cloud"
                        }
                        onChipClosed: function(key) {
                            requestDeleteModel("llmModels", key, "llmActiveModel", "llmModelType")
                        }
                        onCloseBlocked: {
                            var win = ApplicationWindow.window
                            if (win && win.toast) win.toast(qsTr("At least one model must be kept"), "warning")
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

                    // ---- Cloud fields (visible when llmModelType === "cloud") ----
                    GridLayout {
                        Layout.fillWidth: true
                        visible: llmModelType === "cloud"
                        columns: 2
                        rowSpacing: 12
                        columnSpacing: 16

                        TextField {
                            label: qsTr("Vendor Name")
                            text: "DeepSeek"
                            readOnly: true
                            Layout.fillWidth: true
                        }
                        TextField {
                            label: qsTr("Base URL")
                            text: "https://api.deepseek.com/v1"
                            Layout.fillWidth: true
                        }
                        TextField {
                            label: qsTr("Model")
                            text: "deepseek-chat"
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

                    // ---- Local fields (visible when llmModelType === "local") ----
                    ColumnLayout {
                        Layout.fillWidth: true
                        visible: llmModelType === "local"
                        spacing: 12

                        TextField {
                            Layout.fillWidth: true
                            label: qsTr("Ollama Server URL")
                            text: "http://localhost:11434"
                        }

                        RowLayout {
                            Layout.fillWidth: true
                            spacing: 16

                            TextField {
                                Layout.fillWidth: true
                                label: qsTr("Model")
                                text: "qwen2.5"
                            }
                            SelectBox {
                                Layout.fillWidth: true
                                label: qsTr("Auth Method")
                                options: ["Bearer", qsTr("No auth")]
                                currentIndex: 1
                            }
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
                            text: qsTr("98 ms latency")
                            color: Theme.muted
                            font.pixelSize: 12
                            anchors.verticalCenter: parent.verticalCenter
                        }
                    }
                }
            }

            // ---- Card 3: Polish Prompt ----
            // HTML: title + desc + .textarea (min-height 80px, resize vertical)
            Card {
                Layout.fillWidth: true
                visible: activeTab === "polish"
                title: qsTr("Polish Prompt")
                description: qsTr("System default prompt + your custom content. Saved to the Go-managed config.yaml.")

                TextArea {
                    Layout.fillWidth: true
                    text: polishPrompt
                    placeholder: qsTr("Enter the system prompt used to polish ASR results...")
                }
            }
            // ================================================================
            // PERSONAL PROMPTS TAB (2 cards, matches HTML .section[data-tab="personal"])
            // ================================================================

            // ---- Card 1: Quick Prompt Injection ----
            // HTML: title + toggle on same row, desc, then prefix-key select (max 200).
            Card {
                Layout.fillWidth: true
                visible: activeTab === "personal"
                title: qsTr("Quick Prompt Injection")
                description: qsTr("When enabled, prefix key + a custom letter/number injects the matching prompt before the ASR result.")
                headerExtra: [
                    Toggle {
                        checked: quickInjectEnabled
                        onToggled: quickInjectEnabled = checked
                    }
                ]

                RowLayout {
                    Layout.fillWidth: true
                    spacing: 16

                    SelectBox {
                        label: qsTr("Prompt Prefix Key")
                        options: ["Ctrl", "Alt", "Win"]
                        currentIndex: 0
                        Layout.preferredWidth: 200
                        Layout.maximumWidth: 200
                        onSelected: function(index, value) {
                            promptPrefixKey = value
                        }
                    }
                    Item { Layout.fillWidth: true }
                }
            }

            // ---- Card 2: Personal Prompt List ----
            // HTML: title + "+ Add" button on same row, desc, Repeater of prompt-items.
            Card {
                Layout.fillWidth: true
                visible: activeTab === "personal"
                title: qsTr("Personal Prompt List")
                description: qsTr("Each prompt maps to a shortcut key. On injection it replaces the full prompt content.")
                headerExtra: [
                    Button {
                        text: qsTr("+ Add")
                        kind: "primary"
                        small: true
                        onClicked: addPersonalPrompt()
                    }
                ]

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 10

                    Repeater {
                        model: personalPrompts

                        delegate: PromptItem {
                            required property var modelData
                            required property int index

                            Layout.fillWidth: true
                            name: modelData.name
                            keyChar: modelData.key
                            text: modelData.text
                            prefixLabel: promptPrefixKey

                            onNameEdited: function(newName) {
                                updatePersonalPrompt(index, "name", newName)
                            }
                            onKeyEdited: function(newKey) {
                                updatePersonalPrompt(index, "key", newKey)
                            }
                            onTextEdited: function(newText) {
                                updatePersonalPrompt(index, "text", newText)
                            }
                            onDeleteRequested: {
                                deletePersonalPrompt(index)
                            }
                        }
                    }

                    // .shortcut-hint: 11px muted, combo in accent
                    Text {
                        Layout.fillWidth: true
                        Layout.topMargin: 4
                        text: qsTr("Tip: shortcut format is \"prefix key + 0-9 / A-Z\", e.g. %1; if it conflicts with other software the key will be inactive.").arg(
                              "<b style='color:" + Theme.accent + "'>" + promptPrefixKey + " + 1</b>")
                        textFormat: Text.RichText
                        color: Theme.muted
                        font.pixelSize: 11
                        wrapMode: Text.WordWrap
                    }
                }
            }
            // ================================================================
            // QUICK TOOLS TAB (2 cards, matches HTML .section[data-tab="tools"])
            // ================================================================

            // ---- Card 1: Desktop Screenshot ----
            // HTML: modifier (flex 0.6) + key form row, save path field, capture btn.
            Card {
                Layout.fillWidth: true
                visible: activeTab === "tools"
                title: qsTr("Desktop Screenshot")

                ColumnLayout {
                    Layout.fillWidth: true
                    spacing: 12

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 16

                        SelectBox {
                            label: qsTr("Modifier")
                            options: ["Ctrl + Shift", "Ctrl + Alt", "Alt + Shift"]
                            currentIndex: 0
                            onSelected: function(index, value) {
                                screenshotModifier = value
                            }
                            Layout.fillWidth: true
                        }
                        TextField {
                            label: qsTr("Key")
                            text: screenshotKey
                            onTextEdited: screenshotKey = newText
                            Layout.fillWidth: true
                        }
                    }

                    TextField {
                        Layout.fillWidth: true
                        label: qsTr("Save Location")
                        text: screenshotSavePath
                        onTextEdited: screenshotSavePath = newText
                    }

                    Row {
                        Layout.topMargin: 4
                        Button {
                            text: qsTr("Capture Now")
                            kind: "primary"
                            onClicked: {
                                var win = ApplicationWindow.window
                                if (win && win.toast) win.toast(qsTr("Screenshot captured"))
                            }
                        }
                    }
                }
            }

            // ---- Card 2: Data Management ----
            // HTML: "Open data dir" (ghost) + "Clear data" (danger).
            Card {
                Layout.fillWidth: true
                visible: activeTab === "tools"
                title: qsTr("Data Management")
                description: qsTr("Local SQLite database and screenshot cache.")

                Row {
                    spacing: 12
                    Layout.topMargin: 4

                    Button {
                        text: qsTr("Open Data Directory")
                        kind: "ghost"
                        onClicked: {
                            var win = ApplicationWindow.window
                            if (win && win.toast) win.toast(qsTr("Opening data directory..."))
                        }
                    }
                    Button {
                        text: qsTr("Clear Data")
                        kind: "danger"
                        onClicked: clearDataConfirm.open()
                    }
                }
            }

            // bottom spacer (SaveBar height) so content isn't hidden behind it
            Item { Layout.fillWidth: true; Layout.preferredHeight: 70 }
        }
    }

    // bottom save bar (visible on settings page)
    SaveBar {
        onSaveRequested: {
            if (!viewModel) {
                var win0 = ApplicationWindow.window
                if (win0 && win0.toast) win0.toast(qsTr("Settings backend not connected"), "warning")
                return
            }
            // push local UI props into the viewModel, then persist via gRPC.
            pushToViewModel()
            viewModel.save()
            var win = ApplicationWindow.window
            if (win && win.toast) win.toast(qsTr("Settings saved"))
        }
    }

    // add model dialog (shown when "+ Add Model" chip clicked)
    AddModelDialog {
        id: addModelDialog
        parent: Overlay.overlay
        property string targetCategory: "asr"
        onSaved: function(name, provider, deployType, customName) {
            if (!viewModel) return
            var rawKey = customName || name || provider || ("model-" + Date.now())
            var key = rawKey.replace(/\s+/g, "-").toLowerCase()
            var isLocal = deployType === qsTr("Local Model") || deployType === "Local Model"
            var cat = addModelDialog.targetCategory || "asr"
            var displayName = name || customName || rawKey

            // 如果 key 已存在，追加数字后缀（防覆盖已有 provider）
            var provs = viewModel.asrProviders || []
            var suffix = 1
            var baseKey = key
            while (true) {
                var dup = false
                for (var i = 0; i < provs.length; i++) {
                    if (provs[i].key === key) { dup = true; break }
                }
                if (!dup) break
                key = baseKey + "-" + suffix
                suffix++
            }

            viewModel.addProvider(cat, key)
            viewModel.updateProvider(cat, key, {name: displayName, type: isLocal ? "local" : "cloud"})
            viewModel.setActiveProvider(cat, key)
            // 直接同步刷新（C++ 数据已更新，syncFromViewModel 从 C++ 读取最新值）
            syncFromViewModel()
            var win = ApplicationWindow.window
            if (win && win.toast) win.toast(qsTr("Model added: ") + displayName)
        }
    }

    // 文件选择对话框（本地模型路径 Browse 按钮）
    FileDialog {
        id: modelFileDialog
        title: qsTr("Select whisper model (.bin)")
        nameFilters: ["whisper model (*.bin)", qsTr("All files") + " (*)"]
        onAccepted: {
            var path = selectedFile.toString()
            // file URL → 本地路径
            path = path.replace(/^file:\/\//, "").replace(/^\//, "")
            asrLocalModelPath = path
            asrLocalModelName = deriveModelName(path)
        }
    }

    // delete confirm dialog (shown when a model chip's × is clicked).
    // Two-step: × opens this; Confirm runs the actual removal.
    ConfirmDialog {
        id: deleteConfirmDialog
        parent: Overlay.overlay
        heading: qsTr("Delete Model")
        message: qsTr("Remove \"%1\"? Its provider, Base URL, and API Key will be discarded. This cannot be undone.").arg(root.pendingLabel)
        confirmText: qsTr("Delete")
        destructive: true
        onConfirmed: {
            root.removeModel(root.pendingListName, root.pendingKey,
                             root.pendingActiveName, root.pendingTypeName)
            // 同步删除后端 provider
            if (viewModel) {
                var cat = listNameToCategory(root.pendingListName)
                if (cat) viewModel.removeProvider(cat, root.pendingKey)
            }
            var win = ApplicationWindow.window
            if (win && win.toast) win.toast(qsTr("Model deleted: ") + root.pendingLabel)
        }
    }

    // clear-data confirm dialog (Clear Data button). Destructive: wipes the
    // local SQLite DB + screenshot cache. No backend wiring yet.
    ConfirmDialog {
        id: clearDataConfirm
        parent: Overlay.overlay
        heading: qsTr("Clear All Data")
        message: qsTr("This permanently deletes the local database and screenshot cache. This cannot be undone.")
        confirmText: qsTr("Clear Data")
        destructive: true
        onConfirmed: {
            var win = ApplicationWindow.window
            if (win && win.toast) win.toast(qsTr("Data cleared"))
        }
    }
}
