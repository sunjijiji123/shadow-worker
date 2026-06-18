# Shadow Worker — 架构设计

> 一个跑在你电脑里的「行为采集器」:默默记录你的工作(语音/屏幕/应用切换),
> 生成带时间轴的结构化工作日志,通过本地 MCP 暴露给 AI agent,
> 让 agent 帮你生成日报、周报、汇报。
>
> **核心理念**:我只做"agent 做不了的事"——本地行为采集。
> 日报生成、代码工作提炼等,交给 agent(它有自己的 LLM 和技能,如 code-worklog)。

---

## 1. 产品定位

```
                    ┌──────────────────┐
   用户日常工作 ───→ │  Shadow Worker   │ ──→ 结构化工作日志
                    │  (行为采集器)     │       (带时间轴)
                    └────────┬─────────┘
                             │ stdio MCP
                             ▼
                    ┌──────────────────┐
                    │  AI Agent        │ ──→ 日报/周报/汇报
                    │  (ZCode/Claude)  │      (agent 用自己的 LLM)
                    │  + code-worklog  │
                    │    技能补 code    │
                    └──────────────────┘
```

**职责边界**:
- Shadow Worker = 采集器 + 数据源 + MCP 提供者(只做这些)
- Agent = 日报生成 + code 工作提炼 + 用户对话(全部交给 agent)
- **不做**:日报生成 UI、code 采集、AI 汇总

## 2. 技术栈(全部锁定)

| 层 | 选型 | 理由 |
|----|------|------|
| 后台服务 | **Go** | MCP 官方 SDK、SQLite、HTTP/SSE 都是 Go 强项 |
| UI 客户端 | **Qt 6.8 LTS**(开源 LGPL) | 透明浮窗刚需、QML 现代 UI、原生 QtGrpc |
| 进程间通信 | **gRPC**(本机 localhost) | 工具链生成桩、支持 4 种流模式 |
| MCP transport | **stdio** | 本地、无端口、无鉴权、隐私 |
| 数据库 | **SQLite** | 单文件、零服务、查询强(在 AGENTS.md 三方库清单) |
| ASR | **whisper.cpp**(cgo) + 云端 SSE | 本地优先、可离线 |
| VLM | **云端 HTTP**(GLM-4V/Qwen-VL)+ Ollama fallback | 多数据源之一 |

**进程模型**:Go 服务(开机自启,长驻)+ Qt 客户端(可选,用户开关)。
Qt 关了 Go 继续采集;开发者甚至可以只用 MCP 不开 UI。

---

## 3. 目录结构(monorepo)

```
shadow-worker/
├── docs/                          # 文档
│   ├── ARCHITECTURE.md            # ← 本文档
│   ├── grpc-api.md                # gRPC 接口契约(从 proto 生成)
│   └── mcp-api.md                 # MCP 工具契约
│
├── proto/                         # gRPC 接口定义(单一真相源)
│   ├── config.proto               # 配置服务
│   ├── whitelist.proto            # 白名单服务
│   ├── overview.proto             # 概览/状态推送
│   ├── asr.proto                  # 语音识别(含流式)
│   └── collection.proto           # 采集控制
│
├── backend/                       # Go 服务(后台)
│   ├── cmd/
│   │   └── shadow-worker/
│   │       └── main.go            # 入口:启动 gRPC + MCP + 采集
│   ├── internal/
│   │   ├── config/                # YAML 配置加载/保存
│   │   ├── storage/               # SQLite 层(唯一碰 DB 的地方)
│   │   │   ├── schema.go          #   建表/迁移
│   │   │   ├── activity.go        #   activity_segments CRUD
│   │   │   ├── events.go          #   events CRUD
│   │   │   └── whitelist.go       #   app_categories CRUD
│   │   ├── collector/             # 采集引擎(后台常驻)
│   │   │   ├── movement.go        #   帧差检测(调度,截图委托 Qt 或自采)
│   │   │   ├── appdetect.go       #   前台应用识别(GetForegroundWindow)
│   │   │   └── vlm.go             #   VLM 截图理解(HTTP)
│   │   ├── asr/                   # ASR 引擎
│   │   │   ├── engine.go          #   接口
│   │   │   ├── whisper/           #   本地 whisper(cgo)
│   │   │   └── cloud/             #   云端 SSE(小米/智谱/自定义)
│   │   ├── grpcapi/               # gRPC server 实现(从 proto 生成桩)
│   │   │   ├── config_server.go
│   │   │   ├── whitelist_server.go
│   │   │   ├── overview_server.go
│   │   │   └── asr_server.go
│   │   └── mcp/                   # MCP server 实现(4 工具)
│   │       └── server.go
│   ├── go.mod
│   └── go.sum
│
├── client/                        # Qt 客户端(UI)
│   ├── CMakeLists.txt
│   ├── src/
│   │   ├── main.cpp               # 入口:启动 gRPC client + UI
│   │   ├── grpc/                  # gRPC client(从 proto 生成桩)
│   │   │   └── client.h/cpp
│   │   ├── audio/                 # 麦克风采集(QAudioSource)
│   │   │   └── recorder.h/cpp
│   │   ├── hotkey/                # 全局热键(Win32 RegisterHotKey)
│   │   │   └── hotkey.h/cpp
│   │   ├── inject/                # 文本注入(Ctrl+V 到焦点)
│   │   │   └── injector.h/cpp
│   │   ├── window/                # 选窗口交互(复用截图遮罩技术)
│   │   │   └── windowpicker.h/cpp
│   │   └── viewmodels/            # QML ↔ C++ 桥(Q_OBJECT)
│   │       ├── overview_vm.h/cpp
│   │       ├── whitelist_vm.h/cpp
│   │       └── settings_vm.h/cpp
│   ├── qml/                       # QML UI 文件
│   │   ├── main.qml               # 主窗口(加载各页)
│   │   ├── FloatWindow.qml        # 透明浮窗(频谱/状态/气泡)
│   │   ├── OverviewPage.qml       # 概览页
│   │   ├── TimelineView.qml       # timeline 色块
│   │   ├── WhitelistPage.qml      # 白名单卡片管理
│   │   ├── SettingsPage.qml       # 设置页(三组配置)
│   │   └── components/            # 复用组件(卡片/chip/状态灯)
│   └── resources/
│       ├── qml.qrc
│       └── icons/
│
├── scripts/                       # 构建辅助
│   ├── gen_proto.sh                # 一键生成 Go + Qt 的 gRPC 桩
│   └── install_service.bat         # 注册 Go 服务为开机自启
│
├── .gitignore
└── README.md
```

**proto 是单一真相源**:接口定义在 `proto/`,跑 `gen_proto.sh` 同时生成 backend(Go)和 client(Qt)的桩。改接口只改 proto,两端自动同步。

---

## 4. Go / Qt 职责边界(锁定)

```
═══════════════════════════════════════════════════════════
Go 服务(后台,长驻)—— 数据中枢
═══════════════════════════════════════════════════════════
数据层(唯一碰 DB):
  ✓ SQLite 所有读写
  ✓ YAML config 读写
  ✓ 截图文件管理

采集层(Qt 关了也跑):
  ✓ movement 帧差检测
  ✓ 前台应用识别 + 白名单匹配
  ✓ VLM 截图理解
  ✓ ASR 识别(cgo whisper + 云端 SSE)

对外暴露:
  ✓ MCP server(stdio,4 工具)
  ✓ gRPC server(localhost)
  ✓ 状态推送(ServerStreaming)

═══════════════════════════════════════════════════════════
Qt 客户端(可选)—— 交互前端
═══════════════════════════════════════════════════════════
展示: 浮窗(透明)/ 主控台(timeline/概览)/ 设置页
交互(开了才有):
  ✓ 全局热键
  ✓ 麦克风采集 → gRPC stream 给 Go
  ✓ 选窗口交互(加白名单)
  ✓ 区域截图
  ✓ 文本注入(Ctrl+V 到焦点)
  ✓ 托盘菜单
```

**配置归属**:
- Go(YAML/SQLite,走 gRPC):ASR/polish/inject/movement/vlm/mcp/hotkeys/general
- Qt 本地(QSettings,不走 gRPC):麦克风设备 ID、窗口位置、UI 偏好

---

## 5. 数据流(三条主线)

### ① 语音主线(Qt 主导,Go 协作)

```
Qt: 热键(Ctrl+Space) → 录音(QAudioSource 采集 PCM)
Qt: gRPC ClientStreaming → 把 PCM 实时推给 Go
Go: 收 PCM → whisper(cgo)/云端 SSE 识别
Go: 存 events 表(语音事件 + 时间戳 + 文本)
Go: gRPC 回传增量文本 → Qt
Qt: 浮窗气泡显示最终文本
Qt: (若 auto 模式)注入到焦点输入框
Qt: gRPC 告知 Go "已注入" → Go 记 prompt_inject 事件
```

### ② 采集主线(Go 独立)

```
Go 后台常驻(300ms 采样):
  GetForegroundWindow → 进程路径
  查白名单:
    在 → 截活动窗口 → 帧差算活跃度
         → 状态变化时写 activity_segments
         → (定时/VLM 触发时)截图 + VLM 理解 → 写 events
    不在 → 忽略(完全不留痕,隐私)
```

### ③ agent 查询主线(纯 Go)

```
agent 调 MCP get_worklog("2026-06-17")
  → Go 查 SQLite(activity_segments + events)
  → 拼装结构化 JSON(见第 8 节)
  → stdio 返回 agent
agent 自己生成日报(可用 code-worklog 技能补 code 段)
```

---

## 6. DB Schema

### app_categories(白名单)

```sql
CREATE TABLE app_categories (
    path        TEXT PRIMARY KEY,      -- 完整进程路径 C:\...\Cursor.exe
    name        TEXT NOT NULL,         -- 显示名 Cursor
    category    TEXT NOT NULL,         -- coding/office/browser/chat/other
    icon_path   TEXT,                  -- 提取的图标缓存路径(可选)
    added_at    INTEGER NOT NULL       -- unix 秒
);
```

### activity_segments(活动段,timeline 主轴)

```sql
CREATE TABLE activity_segments (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    start_ts        INTEGER NOT NULL,  -- unix 秒
    end_ts          INTEGER NOT NULL,
    app_path        TEXT NOT NULL,     -- 关联 app_categories.path
    app_name        TEXT NOT NULL,     -- 冗余存,查询快
    category        TEXT NOT NULL,     -- 冗余存(类别固定色)
    window_title    TEXT,              -- 窗口标题快照
    state           TEXT NOT NULL      -- active/idle
);
CREATE INDEX idx_segments_start ON activity_segments(start_ts);
CREATE INDEX idx_segments_cat_date ON activity_segments(category, start_ts);
```

### events(点状事件)

```sql
CREATE TABLE events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ts          INTEGER NOT NULL,      -- 时刻
    type        TEXT NOT NULL,         -- voice/prompt_inject/screenshot/vlm_summary
    app_path    TEXT,                  -- 当时前台应用
    app_name    TEXT,
    content     TEXT,                  -- 语音文本/提示词内容/VLM 摘要
    screenshot_path TEXT,              -- 截图文件路径(type=screenshot/vlm_summary 时)
    meta        TEXT                   -- JSON 扩展字段
);
CREATE INDEX idx_events_ts ON events(ts);
CREATE INDEX idx_events_type_ts ON events(type, ts);
```

**说明**:SQLite 文件位于 `%APPDATA%/shadow-worker/data.db`,
截图位于 `%APPDATA%/shadow-worker/screenshots/YYYY-MM-DD/HHMMSS-app.png`。

---

## 7. gRPC 接口契约

> 详细 proto 见 `proto/*.md`,这里列服务清单和调用模式。

### 调用模式分布

| 服务 | 模式 | 用途 |
|------|------|------|
| Config | Unary | Get/Save 配置 |
| Whitelist | Unary | List/Add/Remove/UpdateCategory |
| Overview | Unary + ServerStreaming | Get 概览 / Watch 实时状态推送 |
| Asr.StreamAudio | ClientStreaming | Qt 推 PCM 给 Go |
| Asr.StreamRecognize | Bidirectional | 边推 PCM 边收增量结果 |
| Collection | Unary | 暂停/恢复采集、手动触发 |

### 关键 RPC 示例

```protobuf
// overview.proto
service Overview {
  rpc GetOverview(OverviewRequest) returns (OverviewData);
  rpc WatchOverview(OverviewRequest) returns (stream OverviewUpdate);
  // OverviewUpdate: { today_minutes, status, active_app, ... }
}

// asr.proto
service Asr {
  rpc StreamRecognize(stream AudioChunk) returns (stream AsrResult);
  // AudioChunk: { pcm_bytes }
  // AsrResult: { partial_text, final_text, done }
}
```

**QtGrpc(Qt 6.5+ 原生)** 支持这四种模式,直接在 QML 里调用。

---

## 8. MCP 工具契约(4 个)

> agent 调这些工具,见 `docs/mcp-api.md`。

### ① get_worklog(主力:生成日报)

```
入参: date(必填), category?(可选), app?(可选)
出参:
{
  "date": "2026-06-17",
  "total_active_minutes": 270,
  "segments": [
    { "start": "09:00", "end": "11:00", "app": "Cursor",
      "category": "coding", "minutes": 120,
      "summary": null, "hint": "use code-worklog skill" },
    { "start": "13:00", "end": "14:00", "app": "Chrome",
      "category": "browser", "minutes": 60,
      "summary": "查阅 Qt QML 文档和 SQLite 教程" }
  ],
  "events": [
    { "time": "11:30", "type": "voice",
      "text": "和产品确认了需求边界" }
  ]
}
```

### ② get_summary(聚合统计)

```
入参: date(必填), group_by?(category 默认/app)
出参:
{ "date": "...", "total_active_minutes": 270,
  "groups": [ { "key": "coding", "minutes": 120, "apps": ["Cursor"] } ] }
```

### ③ search_events(关键词搜)

```
入参: query(必填), date?, type?
出参: { "results": [ { date, time, type, text } ] }
```

### ④ list_apps(列白名单)

```
入参: (无) 或 date?
出参: { "apps": [ { path, name, category, today_minutes } ] }
```

**截图不返回**(用户选了"只返摘要文字"),日后要再加独立 `get_screenshot` 工具。

**hint 字段的巧思**:coding 段我返回 `summary: null + hint: "use code-worklog skill"`,
主动提示 agent 去调它的 code-worklog 技能补内容。两个数据源在 agent 侧汇合。

---

## 9. UI 设计要点(详见各 .qml)

### 类别固定色映射(全局统一:卡片/timeline/统计)

```
coding  = #3B82F6 蓝
office  = #8B5CF6 紫
browser = #F59E0B 橙
chat    = #10B981 绿
other   = #6B7280 灰
```

### 设置页结构(三组 + 概览)

```
概览(默认首页): 今日数据 + 状态灯 + 快捷开关
采集组:  📋 采集应用(白名单卡片) / 📊 采集参数 / 📷 屏幕理解(VLM)
输入组:  🎤 语音识别(+🎙音频设备) / ✨ 文本润色 / ⌨️ 注入模式 / ⌨️ 快捷键
系统组:  🔌 MCP 服务 / 📁 数据管理 / ℹ️ 关于
```

- 白名单:卡片式(图标+名+类别chip+今日时长),添加走"选窗口遮罩交互"
- 类别:点 chip 弹 5 选项改;空白开始,添加时分类(默认 other)
- 白名单增删改**即时生效**,表单配置**手动保存**

### 浮窗 + 气泡

```
Preview 模式: 气泡常驻,显示最终文本 + [取消][复制],用户主动关
Auto 模式:    录音完瞬间注入(焦点可信窗口),气泡闪现"✓已注入",3 秒消失
气泡在浮窗上方(右下角垂直堆叠)
润色(on/off)和注入(preview/auto)是两个正交配置
```

---

## 10. 开工顺序(建议)

```
第 1 周: 骨架打通
  1. proto 定义 + gen_proto.sh(生成两端桩)
  2. Go: gRPC server 空壳 + SQLite 建表 + 配置加载
  3. Qt: gRPC client 连通 + 一个最简 QML 页面调通 GetOverview
  → 验收:Qt 能从 Go 拿到一条假数据

第 2 周: 采集闭环
  4. Go: movement + appdetect + 白名单 + activity_segments 写入
  5. Go: MCP server 4 工具(查 SQLite 返回)
  6. Qt: 白名单管理(选窗口交互 + 卡片)+ 概览页
  → 验收:能加白名单、采集、agent 调 MCP 拿到真实数据

第 3 周: 语音主线
  7. Go: ASR(whisper cgo + 云端 SSE)+ events 写入
  8. Qt: 麦克风 + 热键 + 浮窗气泡 + 注入
  9. Qt: timeline 渲染
  → 验收:完整语音→识别→注入闭环 + timeline 可视化

第 4 周: VLM + 打磨
  10. Go: VLM 截图理解(定时/按需)+ 摘要回填
  11. 设置页其余配置项 + 状态推送
  12. 打包 + 开机自启
```

**每个里程碑都有可验收的东西,不憋大招。**
