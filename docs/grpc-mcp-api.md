# Shadow Worker — gRPC & MCP 接口契约

> 本文档是 `proto/` 和 MCP server 的设计契约。
> 详细 proto 定义见 `proto/*.proto`,本文档是"为什么这么设计"和字段语义。

---

## 一、gRPC 接口(Qt 客户端 → Go 服务)

### 设计原则

- **Qt 是瘦客户端**:所有数据操作走 Go,Qt 不直接碰 SQLite/YAML
- **状态用流推送**:今日时长、采集状态这类实时数据,用 ServerStreaming 推,Qt 不轮询
- **PCM 用流传输**:麦克风音频走 ClientStreaming/Bidirectional,不走 Unary(太大)

### 服务清单

#### 1. ConfigService(Unary)

```protobuf
service Config {
  rpc GetConfig(GetConfigRequest) returns (ConfigData);
  rpc SaveConfig(ConfigData) returns (SaveResult);
}
```

**职责**:读写 YAML config(ASR/polish/inject/movement/vlm/mcp/hotkeys/general)。
**注意**:白名单、音频设备不在 ConfigService 里——白名单走 WhitelistService(存 SQLite),
音频设备是 Qt 本地配置(QSettings,不走 gRPC)。

#### 2. WhitelistService(Unary)

```protobuf
service Whitelist {
  rpc List(ListRequest) returns (AppList);
  rpc Add(AddRequest) returns (App);           // Qt 选完窗口后调,传 path/name/category
  rpc Remove(RemoveRequest) returns (Result);
  rpc UpdateCategory(UpdateCategoryRequest) returns (Result);
  rpc PickWindow(PickWindowRequest) returns (App);  // 可选:Go 侧抓前台窗口信息
}
```

**职责**:白名单 CRUD。`Add` 即时生效(不等表单保存)。
**PickWindow 设计选择**:窗口选择交互(遮罩+高亮)在 Qt 侧做(UI),
但抓进程路径(QueryFullProcessImageName)可在 Go 或 Qt 做。建议 Qt 做完遮罩选中后,
自己抓进程路径传给 Go 的 `Add`,这样 `PickWindow` RPC 可不要。

#### 3. OverviewService(Unary + ServerStreaming)

```protobuf
service Overview {
  rpc GetOverview(OverviewRequest) returns (OverviewData);
  rpc WatchOverview(WatchOverviewRequest) returns (stream OverviewUpdate);
  rpc GetHeatmap(HeatmapRequest) returns (HeatmapData);          // v2 新增:活跃热力图
  rpc GetCategoryRank(RankRequest) returns (CategoryRankData);   // v2 新增:类别占比排行
}

message OverviewRequest {
  string date = 1;                    // 默认今天;range=day 时用此
  string range = 2;                   // day(默认)/ week / month,支撑概览页切换
}

message OverviewData {
  int32 today_minutes = 1;            // 当前 range 的工作总分钟
  int32 active_segments = 2;
  repeated AppSummary apps = 3;       // 有活动的应用(应用排行)
  string collection_status = 4;       // running/paused
  string asr_status = 5;
  string vlm_status = 6;
  string mcp_status = 7;
  // v2 新增字段
  int32 interrupt_count = 8;          // 打断次数(见下方定义)
  int32 minutes_delta = 9;            // 较上一周期 ±分钟(今日 vs 昨日)
  int32 interrupt_delta = 10;         // 较上一周期 ±打断次数
  int32 app_count = 11;               // 涉及应用数
  string active_app = 12;             // 当前前台白名单应用(当前应用卡)
  string active_category = 13;        // active_app 的类别
}
```

**打断次数定义(决策 A,简单实现):**
- `interrupt_count` = 当天 `activity_segments` 中 **active↔idle 的切换次数**(即用户离开又回来的次数)。
- SQL 语义:统计 state 字段从 `idle` 变 `active` 的次数(每次"离开后恢复工作"算一次打断)。
- 不计 active→active 的连续段(同类别/同应用续段不算打断)。
- 较昨日对比:`minutes_delta` / `interrupt_delta` = 今天 - 昨天。

```protobuf
message OverviewUpdate {
  int32 today_minutes = 1;            // 实时更新
  string collection_status = 2;
  string active_app = 3;              // 当前前台白名单应用
}
```

**热力图 RPC(v2 新增):**

```protobuf
message HeatmapRequest {
  int32 months_back = 1;              // 向前回溯几个月(默认 3,即显示近 3 个月)
}

message HeatmapData {
  repeated DayActivity days = 1;
}

message DayActivity {
  string date = 1;                    // YYYY-MM-DD
  int32 minutes = 2;                  // 当日活跃分钟
  int32 level = 3;                    // 0~5 档(0=无数据,5=满格),前端直接映射绿深浅
}
```

- 用途:概览页活跃热力图(GitHub 贡献格风格)。
- `level` 由后端按 minutes 分桶(如 0/30/60/120/180/240+ 对应 0~5),前端不自己算。

**类别占比排行 RPC(v2 新增):**

```protobuf
message RankRequest {
  string date = 1;
  string range = 2;                   // day/week/month
}

message CategoryRankData {
  string range = 1;
  int32 total_minutes = 2;
  repeated CategoryStat categories = 3;
}

message CategoryStat {
  string category = 1;                // coding/browser/chat/office/other
  int32 minutes = 2;
  int32 percent = 3;                  // 0~100(四舍五入)
  string color = 4;                   // 类别固定色 #3B82F6 等(前端也可本地映射)
}
```

- 用途:概览页类别占比排行(横条 + 占比% + 时长)。
- 按占比降序返回。

**职责**:概览页数据。`WatchOverview` 持续推送,Qt 自动刷新。`GetHeatmap`/`GetCategoryRank` 在页面加载和 range 切换时调用。

#### 4. AsrService(Bidirectional Streaming)

```protobuf
service Asr {
  rpc StreamRecognize(stream AudioChunk) returns (stream AsrResult);
}

message AudioChunk {
  bytes pcm = 1;                       // 16kHz/mono/int16 PCM
}

message AsrResult {
  string partial_text = 1;             // 增量识别结果(实时显示)
  string final_text = 2;               // 最终结果(录音结束时)
  bool done = 3;                       // 是否结束
}
```

**职责**:Qt 推 PCM,Go 识别并返回增量结果。录音结束时 Go 写 events 表。

#### 5. CollectionService(Unary)

```protobuf
service Collection {
  rpc Pause(PauseRequest) returns (Result);
  rpc Resume(ResumeRequest) returns (Result);
  rpc GetStatus(StatusRequest) returns (CollectionStatus);
}
```

**职责**:采集暂停/恢复。

---

## 二、MCP 接口(Agent → Go 服务)

### 设计原则

- **stdio transport**:agent 通过启动 `shadow-worker.exe --mcp` 子进程,stdin/stdout 通信
- **只返摘要文字**:截图不给 agent(用户选择),日后要再加独立 get_screenshot
- **hint 字段**:coding 段主动提示 agent 调 code-worklog 技能

### 4 个工具

#### ① get_worklog(主力:生成日报)

```
入参:
  date     string  必填  "2026-06-17"
  category string  可选  只返该类别(coding/office/browser/chat/other)
  app      string  可选  只返该应用(按 app_name 匹配)

出参:
{
  "date": "2026-06-17",
  "total_active_minutes": 270,
  "segments": [
    {
      "start": "09:00", "end": "11:00",
      "app": "Cursor", "category": "coding",
      "minutes": 120,
      "summary": null,
      "hint": "use code-worklog skill"        // ← 主动提示 agent
    },
    {
      "start": "13:00", "end": "14:00",
      "app": "Chrome", "category": "browser",
      "minutes": 60,
      "summary": "查阅 Qt QML 文档和 SQLite 教程"   // ← VLM 摘要
    }
  ],
  "events": [
    { "time": "11:30", "type": "voice",
      "text": "和产品确认了需求边界" },
    { "time": "15:00", "type": "prompt_inject",
      "text": "请重构以下代码..." }
  ]
}
```

**场景**:用户对 agent 说"生成今天日报"→ agent 调此工具。

#### ② get_summary(聚合统计)

```
入参:
  date      string  必填
  group_by  string  可选  "category"(默认)/ "app"

出参:
{
  "date": "2026-06-17",
  "total_active_minutes": 270,
  "groups": [
    { "key": "coding", "minutes": 120, "apps": ["Cursor"] },
    { "key": "browser", "minutes": 60, "apps": ["Chrome"] }
  ]
}
```

**场景**:"我昨天在 Cursor 花了多久" → 按应用聚合。

#### ③ search_events(关键词搜)

```
入参:
  query  string  必填  关键词
  date   string  可选  限定某天
  type   string  可选  voice/prompt_inject

出参:
{
  "results": [
    { "date": "2026-06-17", "time": "11:30",
      "type": "voice", "text": "和产品确认了需求边界" }
  ]
}
```

**场景**:"我说过关于 X 的话吗" → 搜语音文本。

#### ④ list_apps(列白名单)

```
入参: (无) 或 date 可选

出参:
{
  "apps": [
    { "path": "C:\\...\\Cursor.exe", "name": "Cursor",
      "category": "coding", "today_minutes": 120 }
  ]
}
```

**场景**:agent 了解"用户在追踪哪些应用"。

---

## 三、配置契约(YAML)

```yaml
# %APPDATA%/shadow-worker/config.yaml

asr:
  mode: cloud                    # cloud | local
  active_provider: xiaomi-mimo
  providers: { ... }             # 同现有 ai-voice-tool

polish:
  enabled: true
  active_provider: deepseek
  providers: { ... }
  prompt: "..."

inject:
  mode: preview                  # preview | auto

movement:
  sample_interval_ms: 300        # 采样间隔
  idle_timeout_s: 10             # 静止超时(判定离开)
  precision: medium              # low | medium | high(映射到帧差阈值)

vlm:
  mode: scheduled                # scheduled | on_demand | off
  active_provider: glm-4v
  providers: { ... }
  schedule_interval_min: 15      # 定时模式下的频率

mcp:
  enabled: true

hotkeys:
  record: "Ctrl+Space"
  screenshot: "Ctrl+Shift+S"
  prompt_prefix: "Ctrl"          # 提示词热键前缀,Ctrl+1/2/3...

general:
  autostart: true                # 开机自启
  collect_on_start: true         # 启动即采集
  data_dir: "%APPDATA%/shadow-worker"
```

**Qt 本地配置(不走 gRPC,存 QSettings)**:
- `audio/device_id`:麦克风设备 ID
- `ui/float_window_pos`:浮窗位置
- `ui/main_window_geometry`:主窗口尺寸/位置
- `ui/last_settings_tab`:上次打开的设置页
