# Shadow Worker — 实现计划(v2 线框稿对齐)

> 本文档基于 2026-06-19 的代码实际盘点 + v2 HTML 线框稿(`ui-wireframes/ui-wireframe-v2.html`)编写。
> **设计真相源 = v2 HTML 线框稿**。docs 旧描述与之冲突处,以 HTML 为准并在 M0 修订 docs。
>
> 配套修订:`docs/ui-spec-v2.md`、`docs/ARCHITECTURE.md` §9、`docs/engineering.md` 路径常量。

---

## 一、后端验证结论(2026-06-19 实测)

### 1.1 真实状态:后端远超 docs 描述,基本可直接联调

| 验证项 | 结果 | 说明 |
|--------|------|------|
| `go build ./...` | ✅ 通过 | **修复了一个阻塞性问题**:5 个 proto 的 Go 桩(`.pb.go` / `_grpc.pb.go`)**从未生成过**,导致 `grpcapi` 包所有 `UnimplementedXxxServer` 等类型 undefined。已用系统 `D:\software\Go\bin\protoc.exe` + `protoc-gen-go` + `protoc-gen-go-grpc` 重新生成 10 个文件。 |
| `go test ./internal/...` | ✅ 全绿 | asr / collector / grpcapi / mcp / storage 全部 ok(config/vlm/winapi 无测试文件) |
| `--mcp` 模式 | ✅ 工作正常 | `initialize` 返回 `serverInfo: shadow-worker v0.1.0`、`protocolVersion 2024-11-05`;`tools/list` 返回 **4 工具完整 schema**:`get_summary` / `get_worklog` / `list_apps` / `search_events`,均带 `inputSchema` + `outputSchema` |

### 1.2 已实现的后端模块(均有代码 + 多数有测试)

```
backend/
├── cmd/shadow-worker/main.go    双模式入口(gRPC 服务 / --mcp)
├── internal/
│   ├── config/      YAML 加载 + default_prompt.txt
│   ├── storage/     schema + activity/events/whitelist/worklog CRUD
│   ├── winapi/      user32/kernel32/gdi32 syscall 封装
│   ├── collector/   appdetect + movement(帧差)+ capture + vlm
│   ├── asr/         engine + cloud + local
│   ├── vlm/         engine + cloud + ollama
│   ├── grpcapi/     5 个 server(Overview/Whitelist/Asr/Collection/Config)
│   └── mcp/         4 工具 + 测试
```

> **结论:后端不是从零写,只需"查漏补缺 + 打磨"。重头戏在 Qt 客户端 UI 重写。**

### 1.3 环境踩坑(重要,影响后续每次构建)

本机 Go 环境变量被污染,导致 `go install` / `go build` 失败:
- **`GOFLAGS`**(系统级)残留了非法字符 `;` → `go install` 报 `all arguments must refer to packages in the same module`
- **`GOPATH`**(系统级)= `D:\software\Go\bin`(末尾多了 `\bin`,且可能带尾随空格)→ 插件装到错误位置

**解决方案(每次跑 go 命令前):**

```bat
cmd /c "set GOPATH=D:\software\Go&& set GOBIN=D:\software\Go\bin&& set GOFLAGS=&& cd /d D:\code\shadow-worker\backend && go build ./..."
```

> 建议把这套环境固化进 `scripts/setenv.bat`(目前该脚本还是旧机器路径 `C:\Users\Administrator\code\1-ai\...`,需更新为 `D:\code\shadow-worker`)。

### 1.4 proto gap 清单(概览页 v2 需求 vs 现有 OverviewData)

对照 v2 概览页逐项核对:

| v2 概览页元素 | 现有 proto/实现 | gap |
|---|---|---|
| 今日工作时长 | `OverviewData.today_minutes` | ✅ 无 |
| 涉及应用数 | `OverviewData.apps[]` 长度 | ✅ 无(但 server.go 现在 `Apps: nil`,需补查询) |
| 当前应用 | `WatchOverview.active_app` | ✅ 无 |
| 采集/ASR/VLM/MCP 状态 | `OverviewData.{collection,asr,vlm,mcp}_status` | ✅ 无 |
| **打断次数** | ❌ 无字段 | **缺** — 需新增 `interrupt_count`(定义:今日 active↔idle 切换次数,或相邻不同类别的 segment 转换次数) |
| **较昨日 ± 对比**(+28min/-3) | ❌ 无历史字段 | **缺** — 需后端补 `yesterday_minutes` / `yesterday_interrupts`,或前端缓存 |
| **今天/本周/本月切换** | `OverviewRequest.date` 只接受单日 | **缺** — 需补 `range` 枚举(day/week/month),或新增 `GetOverviewByRange` |
| **活跃热力图**(多月 GitHub 贡献格) | ❌ 无聚合 RPC | **缺** — 需新增 `GetHeatmap(months_back)` 返回每日活跃分钟(可复用 storage 的按日聚合) |
| **类别占比排行**(今日各类别分钟+占比) | ❌ Qt 无 RPC(MCP `get_summary` 有逻辑但走 stdio) | **缺** — 需在 gRPC 暴露 `GetCategoryRank(date/range)`,复用 storage 聚合 |
| 应用排行 | `OverviewData.apps`(name/category/today_minutes) | ✅ 无(server.go 需补填充) |
| 快捷开关(采集/自启/启动即采集) | `CollectionService.Pause` + `ConfigData` | ✅ 无 |

**后端待补的 RPC(预估 0.5 天):**

1. `OverviewRequest` 加 `range` 字段(`day`/`week`/`month`,默认 day)→ `OverviewData` 按范围聚合
2. `OverviewData` 加 `interrupt_count` + `interrupt_delta` + `minutes_delta`(历史对比)
3. 新增 `OverviewService.GetHeatmap(HeatmapRequest)` → `repeated DayActivity{date, minutes, level}`
4. 新增 `OverviewService.GetCategoryRank(RankRequest)` → `repeated CategoryStat{category, minutes, percent}`
5. `server.go` 的 `GetOverview` 把 `Apps` 填充上(目前 nil)

---

## 二、客户端现状(最大问题)

现有 QML 是按**旧设计**写的,与 v2 线框稿严重不一致,需重写:

| 维度 | 现有 QML | v2 要求 |
|------|----------|---------|
| 导航 | `ToolBar` + 4 个文字按钮 + `StackView` | **180px 侧边栏 + SVG 图标 + active 态(绿底+左框)+ 4 项:概览/时间轴/设置/系统** |
| 概览页 | 简单 stats + 应用列表 | stats-row **4 卡** + 采集应用横滑卡 + **热力图** + 快捷开关卡 + **类别占比排行** + 应用排行 + 今天/本周/本月切换 |
| 时间轴页 | 基础 timeline | **日期选择器(带日历弹窗)+ 今天按钮** + 单焦点 track + 工作记录/事件列表双 tab + 类别筛选 |
| 设置页 | 简单表单 | **6 tab**(语音输入/行为采集/图像识别/文字润色/个人提示词/快捷工具)+ 模型 chip + 添加模型弹窗 + 底部统一保存条 + toast |
| 系统页 | **完全缺失** | 关于 / 开机自启 / MCP 服务(4 工具展示+三 tab 复制配置)/ 软件更新 / 数据管理 / 配置路径 |
| 录音浮窗 | 单状态 | **完整状态机**(listening 频谱 / transcribing 滚动字 / polishing 蓝点 / completed ✓)+ 结果气泡(AI 润色图标+可编辑+Enter注入) |

---

## 三、完整实现计划(4 个里程碑,每步可验收)

> 已确认的取舍(用户拍板):
> - **时间轴页只保留日视图 + 日期选择**(周/月能力只在概览页统计体现,贴合当前 HTML)
> - **先验证后端**(已完成,见第一节),后端基本就绪

### M0 — 文档对齐(0.5 天)

把过时 docs 与 v2 HTML 对齐,确保后续施工单一真相源清晰。

- [ ] `docs/ui-spec-v2.md`
  - §2.2 设置 tab:**5 个 → 6 个**(语音输入/行为采集/图像识别/文字润色/个人提示词/快捷工具),补"行为采集"为独立 tab
  - §3 概览页:**重写** — 4 stats 卡(今日时长/打断次数/涉及应用/当前应用)+ 采集应用横滑卡 + 热力图 + 快捷开关 + 类别占比排行 + 应用排行 + 今天/本周/本月切换
  - §4 时间轴页:**删掉** §4.3 周视图 / §4.4 月视图静态网格描述,改为"日期选择器(日历弹窗)驱动单日视图"
  - §5 设置页 tab 数对齐(原写的 5 个补成 6 个)
- [ ] `docs/ARCHITECTURE.md` §9:设置结构从"三组"改为"6 tab",footer 改"统一保存条"
- [ ] `docs/engineering.md` §一:路径常量 `C:\Users\Administrator\code\1-ai\...` → `D:\code\shadow-worker`;补 GOFLAGS/GOPATH 污染坑 + 干净构建命令
- [ ] `docs/grpc-mcp-api.md`:OverviewData 补 `interrupt_count` / delta / range 字段;新增 `GetHeatmap` / `GetCategoryRank`

**验收:** docs 与 HTML 逐项一致,无矛盾。

---

### M1 — 后端 proto 补全(0.5 天)

补第一节 1.4 列出的 proto gap,让 Qt 有数据可渲染。

- [ ] `proto/overview.proto`:
  - `OverviewRequest` 加 `string range = 2;` (day/week/month)
  - `OverviewData` 加 `int32 interrupt_count` / `int32 interrupt_delta` / `int32 minutes_delta` / `repeated AppSummary apps`(server 填充)
  - 新增 `GetHeatmap(HeatmapRequest) returns (HeatmapData)` + message `DayActivity{date, minutes, level}`
  - 新增 `GetCategoryRank(RankRequest) returns (CategoryRankData)` + `CategoryStat{category, minutes, percent, color}`
- [ ] 重新 `gen_proto` 生成 Go 桩(注意干净 env)
- [ ] `backend/internal/storage/`:补 `InterruptCount(date)` / `DailyMinutesRange(start,end)` / `CategoryAggregate(date/range)` 查询
- [ ] `backend/internal/grpcapi/server.go`:`GetOverview` 填充 Apps + 新 RPC 实现
- [ ] `go test` 保持绿

**验收:** `grpcurl` 或 Qt 调 `GetOverview` 能拿到打断次数;`GetHeatmap` 返回每日分钟;`GetCategoryRank` 返回类别占比。

---

### M2 — Qt 客户端 UI 骨架重写(1.5 天)

把旧 QML 推倒,按 v2 搭新骨架。**所有页先接通 gRPC 数据 + 卡片骨架,细节填充留 M3/M4。**

- [ ] `client/qml/theme/Theme.qml` 单例:8 个 CSS 变量(--bg/--bg2/--bg3/--ink/--muted/--rule/--accent/--danger)→ QML 属性,全局引用
- [ ] 重写 `main.qml`:`ApplicationWindow` + 180px 侧边栏(4 项 SVG nav + active 态)+ 内容区 view 切换 + 系统托盘
- [ ] `qml/components/` 抽通用组件:`Card` / `StatCard` / `Chip` / `ModelChip` / `Toggle` / `Radio` / `Toast` / `SaveBar` / `ChipFilter`
- [ ] 4 个 view 骨架:`OverviewPage` / `TimelinePage` / `SettingsPage` / `SystemPage`,各自接对应 ViewModel
- [ ] ViewModel:`overview_vm`(补打断/热力图/rank 查询)、`timeline_vm`(接 `QueryTimeline`)
- [ ] CMake:`qt_add_protobuf` / `qt_add_grpc` 自动生成 Qt 桩(确认 CMakeLists 已配)

**验收:** Qt 连上 Go,侧边栏切 4 页不崩,配色与 HTML 一致,概览页能显示真实 today_minutes。

---

### M3 — 概览页 + 时间轴页完整还原(2 天)

把 v2 HTML 这两个页 1:1 还原到 QML。

**概览页:**
- [ ] stats-row 4 卡(今日时长 / 打断次数 / 涉及应用 / 当前应用)+ 较昨日 delta
- [ ] 采集应用横滑卡片(图标+名+类别,末尾"+ 添加"→跳设置白名单)+ "管理"按钮
- [ ] 活跃热力图(GitHub 贡献格风格,多月横向滚动,hover tooltip 显示日期+分钟)
- [ ] 快捷开关卡(采集状态 / 开机自启 / 启动即采集)+ 概览页 toggle 自动保存 toast
- [ ] 类别占比排行(横条 + 占比% + 时长)
- [ ] 应用排行(横条 + 时长)
- [ ] 今天/本周/本月 chip 切换 → 触发 `GetOverview(range)`

**时间轴页:**
- [ ] 日期选择器(`<` `日期` `>` + 点击展开日历弹窗)+ "今天"按钮
- [ ] 日历弹窗(月份切换 + 今日高亮 + 有数据日期绿点)
- [ ] 单焦点 timeline track(类别色块拼接,idle 灰间隙,hover tooltip)
- [ ] ruler 时间刻度
- [ ] 工作记录 / 事件列表 双 tab
- [ ] 工作记录:类别筛选 chip + seg-row 列表(时间/应用/类别badge/时长/VLM摘要)
- [ ] 事件列表:类型筛选 chip + app-row(时间/类型/内容/元信息)

**验收:** 对比 HTML 与 Qt 截图,布局/配色/数据一致;切换日期、类别筛选均生效。

---

### M4 — 设置页(6 tab)+ 系统页 + 浮窗状态机(2 天)

**设置页 6 tab:**
- [ ] **行为采集**:白名单卡片(图标+名+类别chip组+移除)+ 采集规则(锁屏暂停toggle / 空闲超时阈值)+ "+ 添加"(选窗口遮罩)+ "扫描应用"
- [ ] **语音输入**:录音热键(hold/press radio + 修饰键+按键)+ ASR 模型服务(chip + 添加弹窗 + 云端/本地表单切换 + 测试连接)+ 音频设备(下拉+测试麦克风+音量条)
- [ ] **图像识别**:VLM 开关(定时/按需 radio + 间隔/热键)+ VLM 模型服务(chip+弹窗+表单+测试)+ 画面采集范围(整个屏幕/仅活动窗口)+ 采集参数(采样间隔/空闲超时/精度)
- [ ] **文字润色**:自动润色 toggle + LLM 模型服务(chip+弹窗+表单+测试)+ 润色提示词 textarea
- [ ] **个人提示词**:快捷注入 toggle + 前缀键 + 提示词列表(名称+快捷键+内容+增删)
- [ ] **快捷工具**:桌面截图(修饰键+按键+保存位置+立即截图)+ 数据管理(打开目录/清空)
- [ ] 底部统一保存条(无重置)+ 全局 toast("已保存")
- [ ] 添加模型弹窗(显示名/服务商/部署类型/自定义名)+ 测试连接走后端 health
- [ ] 白名单增删**即时生效**,表单配置**手动保存**

**系统页:**
- [ ] 关于(版本+仓库地址)
- [ ] 开机自启 toggle
- [ ] MCP 服务(状态灯 + 重启按钮 + 4 工具展示 + Claude/Cursor/原始 JSON 三 tab + 复制按钮)
- [ ] 软件更新(版本+徽章+官方/GitHub/日志/检查更新+启动检查/每日检查 toggle+服务器地址)
- [ ] 数据管理(打开目录/清空所有记录)
- [ ] 配置文件路径(只读 input)

**录音浮窗完整状态机:**
- [ ] 重写 `Bubble.qml`:`listening`(频谱)→ `transcribing`(文字右→左滚动)→ `polishing`(蓝色跳动点)→ `completed`(✓)
- [ ] 结果气泡:可编辑 textarea + AI 润色图标(自动润色高亮不可点 / 关闭时灰可点 / 手动润色后高亮不可回退)+ 润色 loading 蒙层
- [ ] Enter 注入 / Esc 关闭 / 复制 按钮
- [ ] 录音热键(hold/press)+ 提示词快捷键(Ctrl+0-9/A-Z)+ 文本注入(Ctrl+V 到焦点)

**验收:** 设置页 6 tab 全部可填可存(→ Go config.yaml);白名单即时生效;系统页 MCP 配置一键复制;浮窗状态机逐态对比 HTML;完整语音→识别→润色→注入闭环。

---

### M5 — 打包 + 开机自启(0.5 天)

- [ ] `windeployqt --qmldir qml` 部署 Qt 依赖
- [ ] `package/ShadowWorker.iss` Inno Setup 打包(含 Go 服务 + Qt 客户端)
- [ ] Go 服务注册为 Windows 开机自启(复用 `client/src/utils/autostart.cpp`)
- [ ] 托盘菜单(显示主窗口 / 暂停采集 / 退出)

**验收:** 双击安装包,装完开机自启,完整功能可用。

---

## 四、总工期与优先级

| 里程碑 | 工期 | 优先级 | 依赖 |
|--------|------|--------|------|
| M0 文档对齐 | 0.5d | 高 | 无 |
| M1 后端 proto 补全 | 0.5d | 高 | M0 |
| M2 Qt UI 骨架 | 1.5d | 高 | M1 |
| M3 概览+时间轴 | 2.0d | 高 | M2 |
| M4 设置+系统+浮窗 | 2.0d | 高 | M2 |
| M5 打包自启 | 0.5d | 中 | M3+M4 |
| **合计** | **≈ 7 天** | | |

**关键路径:** M0 → M1 → M2 → M3/M4(可并行)→ M5

---

## 五、关键决策记录

1. **设计真相源 = v2 HTML 线框稿**(2026-06-19 确认)。docs 旧描述冲突处以 HTML 为准。
2. **时间轴页只保留日视图 + 日期选择**,周/月能力只在概览页统计体现(用户 2026-06-19 拍板)。
3. **后端先验证**:已确认 go build / go test / MCP 全绿,后端可直接联调(2026-06-19)。
4. **Qt 客户端需重写**(旧 QML 是旧设计,与 v2 不一致),不是增量改造。
5. **环境坑**:本机 GOFLAGS/GOPATH 系统级变量被污染,go 命令前必须 `set GOPATH=...&& set GOBIN=...&& set GOFLAGS=`,见 §1.3。
