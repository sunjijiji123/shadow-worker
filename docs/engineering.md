# 工程规范 + Trae 交接指南

> Shadow Worker 的代码规范、构建环境、和给 Trae AI IDE 接手的工作流。
> **Trae 照这份文档 + 其他 docs/ 文档实现,不需要额外上下文。**

---

## 一、构建环境(关键!不配对就编译不过)

### 已踩的坑(详见 git log)

1. MSYS2 MinGW 污染 MSVC → 必须清理 PATH
2. WrapProtoc 找不到 protoc → protoc 必须在 PATH 最前
3. Qt6Grpc clean configure 失败 → 用增量构建,不要轻易 clean
4. **Go 环境变量被污染(2026-06-19 发现)**:本机系统级 `GOFLAGS` 残留非法字符 `;`、`GOPATH` 被设成 `D:\software\Go\bin`(多了 `\bin`),导致 `go install` / `go build` 失败。**每次跑 go 命令前必须在同一进程内 set 干净**(见下"干净构建命令")。`go env -w` 无法覆盖系统级环境变量(会有 warning)。

### 干净构建命令(Go,必须每次带这套 set)

```bat
cmd /c "set GOPATH=D:\software\Go&& set GOBIN=D:\software\Go\bin&& set GOFLAGS=&& cd /d D:\code\shadow-worker\backend && go build ./..."
```

> 原因:`cmd /c` 启动的子进程能继承 set 的值;`&&` 必须紧贴(避免尾随空格污染)。
> 这套环境已固化进 `scripts/setenv.bat`(下方),source 后可直接 `go build`。

### `scripts/setenv.bat`(固化环境,每次开发前 source)

```bat
@echo off
REM === Go 环境清理(本机 GOFLAGS/GOPATH 被污染,必须覆盖)===
set "GOPATH=D:\software\Go"
set "GOBIN=D:\software\Go\bin"
set "GOFLAGS="

REM === Qt + MSVC + protoc(清理 MSYS2/MinGW)===
set "PATH=C:\Windows\System32;C:\Windows"
call "C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools\VC\Auxiliary\Build\vcvars64.bat"
set "PATH=D:\software\Go\bin;C:\Qt\6.11.1\msvc2022_64\bin;C:\Qt\Tools\CMake_64\bin;C:\Qt\Tools\Ninja;%PATH%"
echo env ready
```

### 关键路径常量(当前机器)

```
项目根:      D:\code\shadow-worker
Go:          D:\software\Go(GOROOT,1.26.x)
Go 工具链:   D:\software\Go\bin(protoc / protoc-gen-go / protoc-gen-go-grpc 都在这)
Qt6 MSVC:    C:\Qt\6.11.1\msvc2022_64
Qt Tools:    C:\Qt\Tools\(CMake_64, Ninja, mingw1310_64)
MSVC:        C:\Program Files (x86)\Microsoft Visual Studio\2022\BuildTools
protoc:      D:\software\Go\bin\protoc.exe(不再用 tools\protoc,该目录已空)
```

### 构建命令

```bash
# Go 后端(先 source scripts\setenv.bat,或用上面的干净命令)
cd backend && go build ./... && go run cmd/shadow-worker/main.go

# Qt 客户端(必须先 source setenv.bat)
cd client && cmake --build build
# 部署依赖(改完 QML 重新部署)
cd build && windeployqt --qmldir ..\qml shadow-worker-client.exe
```

### 生成 gRPC 桩(改 proto 后跑)

```bash
# Go 桩(注意:用 D:\software\Go\bin\protoc.exe,不是旧路径 tools\protoc)
cd D:\code\shadow-worker
D:\software\Go\bin\protoc.exe -I proto ^
  --go_out=backend/internal/grpcapi --go_opt=paths=source_relative ^
  --go-grpc_out=backend/internal/grpcapi --go-grpc_opt=paths=source_relative ^
  proto\asr.proto proto\collection.proto proto\config.proto proto\overview.proto proto\whitelist.proto

# 生成产物被 .gitignore 忽略(*.pb.go / *_grpc.pb.go),不入库
# Qt 桩:CMake qt_add_protobuf/qt_add_grpc 自动生成,不用手动跑
```

> **首次安装 grpc 插件**(机器没装过时):
> ```bat
> cmd /c "set GOPATH=D:\software\Go&& set GOBIN=D:\software\Go\bin&& set GOFLAGS=&& go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1"
> ```
> `protoc-gen-go` 已在 `D:\software\Go\bin`。

---

## 二、代码规范

### Go(后端)

- Go 1.21+, `gofmt` 格式化
- 包结构:`cmd/shadow-worker/` 入口, `internal/{config,storage,collector,asr,grpcapi,mcp,winapi}/` 模块
- 错误处理:返回 `error`,不 panic;上层 gRPC handler 转 `status.Errorf`
- 日志:`log/slog`(标准库),JSON 格式,写文件 `%APPDATA%/shadow-worker/service.log`
- 命名:驼峰,导出大写,内部小写
- 测试:关键模块(movement 帧差、appdetect、storage 查询)要有 `*_test.go`

### C++(Qt 客户端)

- C++20,Qt 6.11
- 类命名:CamelCase(`OverviewViewModel`);成员 m_ 前缀(`m_todayMinutes`)
- ViewModel 模式:每个 QML 页对应一个 `*ViewModel`(继承 QObject,Q_PROPERTY + Q_INVOKABLE)
- QML 文件名:CamelCase(`OverviewPage.qml`)
- 头文件:#pragma once(不用 #ifndef)
- 字符集:UTF-8 无 BOM(`AGENTS.md` 强制)

### QML

- 4 空格缩进
- 属性顺序:id → 属性 → 信号 → 函数 → 子元素
- 组件抽离:`qml/components/`(卡片、chip、状态灯)

### 语言与 i18n(全局强制)

> **代码里不写中文** —— 注释、字符串、UI 显示文字一律用英文。中文通过翻译文件提供。
> 这条是硬规范,避免编码事故(如 PowerShell `Set-Content` 用 GBK 损坏 UTF-8),也是正式产品的标准做法。

- **所有源码(Go / C++ / QML / proto)注释**:英文
- **所有字符串字面量(UI 文字、日志、错误信息)**:英文
  - QML 用 `qsTr("...")` 包裹英文源串,如 `text: qsTr("Overview")`
  - Go 用英文字符串(日志、error)
- **中文显示**:走 Qt Linguist 翻译链
  - 源:`client/i18n/shadow-worker_zh_CN.ts`(XML,人工/linguist 编辑)
  - 编译:`lrelease` → `shadow-worker_zh_CN.qm`
  - 加载:`main.cpp` 装 `QTranslator`,按系统 locale 选 zh_CN
  - CMake:`qt_add_translations()` 或 lrelease 规则生成 .qm
- **类别/事件中文名等枚举映射**:不放代码里,放翻译文件(英文 key → 中文 value)
- **禁止**:在 .go/.cpp/.qml/.proto 里直接写中文字符(注释也不行)

> 例外:`docs/*.md`(文档面向中文读者)和 `README.md` 可以中文。

### proto

- 文件名:小写下划线(`overview.proto`)
- message:CamelCase;字段:snake_case
- 每个 service 单独一个 proto 文件
- `option go_package` 必填

---

## 三、错误处理规范

```
Go 后端:
  Win32 调用失败 → 返回 error,记 slog.Warn,跳过本次采样
  gRPC handler 错误 → status.Errorf(codes.Internal, ...)
  采集线程 panic → recover,记 slog.Error,继续下个采样(不崩服务)

Qt 客户端:
  gRPC 错误 → ViewModel setError(),QML 显示红字
  UI 异常 → try/catch,记 qDebug()
```

---

## 四、日志规范

```
Go: log/slog,JSON 格式
  service.log: {"time":"...","level":"INFO","msg":"service started","port":50051}
  轮转:按天,保留 7 天

Qt: QtLoggingCategory
  service.log(QFile): 输出关键事件(连接 gRPC、录音开始/结束)
  开发时打开 qml category 看详细日志
```

---

## 五、Trae 接手工作流

### Trae 项目初始化

1. **打开 shadow-worker 目录**(Trae 作为 workspace)
2. **把 docs/ 全文喂给 Trae**(让它读 ARCHITECTURE/win32-api/mcp-impl/本文档)
3. **每次开发会话前**:Trae 在终端跑 `scripts\setenv.bat`

### Trae 任务清单(按周)

#### 第 2 周:采集闭环

```
任务 2.1:internal/storage/ 实现 SQLite schema + 4 表 CRUD
  输入:docs/ARCHITECTURE.md 第 6 节 schema
  输出:schema.go(建表) + activity.go/events.go/whitelist.go(CRUD)
  验收:go test ./internal/storage/ 通过

任务 2.2:internal/winapi/ Win32 封装
  输入:docs/win32-api.md 第 1 节(appdetect)
  输出:user32.go/kernel32.go/gdi32.go(syscall 声明)+ appdetect.go
  验收:go run 测试 ForegroundApp() 返回当前前台应用

任务 2.3:internal/collector/ movement 采集
  输入:docs/win32-api.md 第 2/4/6 节
  输出:movement.go(FrameDiff + 主循环)+ capture.go
  验收:启动后台服务,前台切 Cursor 2 分钟,DB 里出现 activity_segment

任务 2.4:internal/mcp/ 4 工具
  输入:docs/mcp-impl.md
  输出:server.go + worklog.go(装配逻辑)
  验收:--mcp 模式启动,ZCode 配置后能调 get_worklog 拿到 JSON

任务 2.5:Qt 白名单管理(选窗口交互)
  输入:docs/win32-api.md 第 3 节
  输出:WindowPicker + WhitelistPage.qml + WhitelistViewModel
  验收:设置页点"添加应用",选 Cursor,DB 里出现记录
```

#### 第 3-4 周:语音主线 + VLM(任务清单类似格式,Trae 照 ARCHITECTURE.md 第 10 节排)

### 给 Trae 的提示词模板

```
任务:[上面某个任务的标题]
输入文档:docs/[对应文件]
约束:
  - 遵守 docs/engineering.md 的代码规范
  - 不破坏现有编译(改完跑 go build / cmake --build)
  - 提交前 gofmt + clang-format
验收:[上面的验收标准]

请实现,完成后告诉我改了哪些文件。
```

---

## 六、现有资产清单(Trae 参考)

```
shadow-worker/
├── docs/
│   ├── ARCHITECTURE.md       完整架构(定位/栈/数据流/UI/开工顺序)
│   ├── grpc-mcp-api.md       gRPC + MCP 接口契约
│   ├── win32-api.md          Win32 采集接口(给 Go 实现用)
│   ├── mcp-impl.md           MCP server 实现规范
│   └── engineering.md        本文档
├── proto/                    5 个 .proto(overview/whitelist/config/asr/collection)
├── backend/                  Go 骨架(main + overview server 已跑通)
├── client/                   Qt 骨架(编译跑通,概览页连上 Go)
├── tools/protoc/             protoc v25.1
└── scripts/                  setenv.bat / build 脚本
```

### 可参考的现有代码

- `ai-voice-tool/`(隔壁项目,如存在)的 Win32 采集、provider 配置、Qt 截图窗口
- `ai-voice-tool/internal/floatbridge/bridge.go` 的 Go syscall 风格
- `ai-voice-tool/floatwindow/screenshotwindow.cpp` 的 Qt 全屏遮罩实现
- `ai-voice-tool/internal/config/config.go` 的 YAML 配置结构

> 注:旧文档引用的 `tools/protoc/` 路径在本机已不存在,protoc 改用 `D:\software\Go\bin\protoc.exe`(见 §一)。
