# Win32 采集接口规范

> Go 后台服务调用 Win32 API 实现:运动检测、前台应用识别、白名单选窗口、活动窗口截图。
> Go 通过 `golang.org/x/sys/windows` 和 `syscall` 调用 Win32。
>
> **本文档给 Trae/AI 看的精确接口契约**,照着实现即可。

---

## 1. 前台应用识别(appdetect)

**目标**:获取当前前台窗口的完整进程路径 + 窗口标题,用于白名单匹配和 activity_segments 记录。

### 核心函数签名(Go)

```go
// internal/collector/appdetect.go

// ForegroundApp 返回当前前台应用信息。
// 失败时返回 error,上层应跳过本次采样。
func ForegroundApp() (App, error)

type App struct {
    Path        string // 完整进程路径 C:\Users\...\Cursor.exe
    Name        string // 进程名 Cursor.exe(从 Path 取末段)
    WindowTitle string // 窗口标题 "main.go - Cursor"
    PID         uint32
}
```

### 实现步骤(参考 ai-voice-tool 现有代码风格)

```go
// 1. 拿前台窗口句柄
hwnd := user32.GetForegroundWindow()
if hwnd == 0 { return error }

// 2. 拿 PID
var pid uint32
user32.GetWindowThreadProcessId(hwnd, &pid)

// 3. 拿完整进程路径(注意:QueryFullProcessImageName 需要 PROCESS_QUERY_LIMITED_INFORMATION 权限)
//    kernel32.OpenProcess(pid) → kernel32.QueryFullProcessImageNameW
hProc := kernel32.OpenProcess(syscall.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
defer kernel32.CloseHandle(hProc)
var pathBuf [syscall.MAX_PATH]uint16
size := uint32(len(pathBuf))
kernel32.QueryFullProcessImageNameW(hProc, 0, &pathBuf[0], &size)
fullPath := windows.UTF16ToString(pathBuf[:size])

// 4. 拿窗口标题
var titleBuf [512]uint16
len := user32.GetWindowTextW(hwnd, &titleBuf[0], int32(len(titleBuf)))
title := windows.UTF16ToString(titleBuf[:len])

// 5. 进程名 = filepath.Base(fullPath)
```

### Win32 函数封装位置

`internal/winapi/` 包(新建),集中放 user32/kernel32 的 syscall 声明:

```go
// internal/winapi/user32.go
package winapi

import "syscall"

var (
    moduser32 = syscall.NewLazyDLL("user32.dll")
    procGetForegroundWindow = moduser32.NewProc("GetForegroundWindow")
    // ... 其他
)
```

---

## 2. 运动检测(movement)

**目标**:每 `sample_interval_ms`(默认 300ms)截活动窗口画面 → YUV 帧差 → 算活跃度 → 判断 active/idle。

### 帧差算法(源自用户的 vga_detect.cpp,适配 Go)

```go
// internal/collector/movement.go

// FrameDiff 比较两帧 RGB(降采样到 320x180),返回变化像素比例。
// ratio > 0.002(0.2%) → 判定有变化。
func FrameDiff(prev, curr []byte, w, h int) float64 {
    var changed int
    thresh := uint8(30) // 精度 medium 的阈值,low=50 high=15
    // 跳过上下边缘 5%~90%,只扫中间(去 OSD/时间戳噪声)
    for i := w*h*5/100; i < w*h*90/100; i++ {
        p := i * 3 // RGB
        if abs(int(prev[p])-int(curr[p])) > thresh ||
           abs(int(prev[p+1])-int(curr[p+1])) > thresh ||
           abs(int(prev[p+2])-int(curr[p+2])) > thresh {
            changed++
        }
    }
    return float64(changed) / float64(w*h)
}

// 精度档位映射
var precisionThresh = map[string]uint8{
    "low": 50, "medium": 30, "high": 15,
}
```

### 采样主循环

```go
func (c *Collector) movementLoop() {
    var prevFrame []byte // RGB 320x180
    var lastChange time.Time

    for {
        select {
        case <-c.pauseCh: continue
        case <-time.After(c.cfg.SampleInterval): // 300ms
        }

        // 1. 前台应用(不在白名单就跳过,不留痕)
        app, _ := appdetect.ForegroundApp()
        if !c.whitelist.Contains(app.Path) { continue }

        // 2. 截活动窗口画面(降采样 320x180 RGB)
        curr := c.captureWindow(app.HWND) // 见第 4 节

        // 3. 帧差
        if prevFrame != nil {
            ratio := FrameDiff(prevFrame, curr, 320, 180)
            if ratio > 0.002 { lastChange = time.Now() }
        }
        prevFrame = curr

        // 4. active/idle 判定(idle_timeout_s 默认 10s)
        active := time.Since(lastChange) < time.Duration(c.cfg.IdleTimeout)*time.Second

        // 5. 状态变化时写 activity_segments(详见 DB schema)
        c.updateSegment(app, active)
    }
}
```

---

## 3. 白名单选窗口交互(Qt 侧)

**目标**:Qt 设置页点"添加应用" → 全屏变暗 → 鼠标悬停窗口高亮 → 点击选中 → 抓进程路径。

### 关键 Win32 API(Qt 通过 windows 调用)

```
全屏遮罩(已有 ai-voice-tool screenshotwindow.cpp 的 Qt 实现,可移植):
  透明 frameless 全屏窗口 + 鼠标事件捕获

鼠标下窗口识别:
  WindowFromPoint(POINT pt) → HWND
  → 同 appdetect 流程拿进程路径 + 标题

窗口高亮:
  GetWindowRect(hwnd) → 在遮罩上画蓝框(QPainter)
  或 SetWindowPos + 边框窗口
```

### Qt 侧函数签名(C++)

```cpp
// client/src/window/windowpicker.h

// WindowPicker: 全屏遮罩选窗口交互
// 用法: pick() 弹遮罩,用户点窗口后 emit picked
class WindowPicker : public QWidget {
    Q_OBJECT
public:
    void pick();  // 进入选窗模式
signals:
    void picked(const QString &path, const QString &title);
    void cancelled();
};
```

实现要点:
1. `pick()` 显示一个全屏透明 QWidget(frameless + 置顶)
2. `mouseMoveEvent` → `WindowFromPoint` 拿鼠标下窗口 → `GetWindowRect` 画高亮框
3. `mousePressEvent` → 抓进程路径(同 appdetect 的 QueryFullProcessImageName 流程) → emit picked
4. `keyPressEvent` ESC → emit cancelled

**进程路径抓取的 Win32 调用在 Qt 侧**:用 `QWindow::fromWinId` 或直接 `windows.h`。可复用 Go 侧 appdetect.go 的逻辑翻译。

---

## 4. 活动窗口截图(captureWindow)

**目标**:截指定 HWND 的画面(降采样到 320x180 RGB),供 FrameDiff 用。
**注意**:VLM 截图用全分辨率(见第 5 节),movement 用降采样。

```go
// internal/collector/capture.go

// CaptureWindow 截指定窗口,返回降采样后的 RGB 字节(320x180x3)。
func CaptureWindow(hwnd syscall.Handle) []byte {
    // 1. GetWindowRect 拿窗口尺寸
    // 2. BitBlt 到内存 DC(GetDC(hwnd) + CreateCompatibleDC + BitBlt)
    // 3. GetDIBits 拿 RGB 像素
    // 4. 降采样到 320x180(双线性或最邻近)
    // 5. 返回 []byte(长度 320*180*3)
}
```

**注意**:某些应用(如 Electron/UWP)BitBlt 会截到黑屏,要用 `PrintWindow(hwnd, hdc, PW_RENDERFULLCONTENT)`。可加 fallback。

---

## 5. VLM 截图(全分辨率)

**目标**:VLM 触发时(定时/按需)截活动窗口,存 PNG 到 `%APPDATA%/shadow-worker/screenshots/YYYY-MM-DD/HHMMSS-app.png`。

```go
// internal/collector/screenshot.go

// CaptureForVLM 截活动窗口全分辨率 PNG,返回文件路径。
func CaptureForVLM(app appdetect.App) (string, error) {
    hwnd := appdetect.ForegroundHwnd() // 复用
    // 1. PrintWindow + GetDIBits 拿全分辨率 RGB
    // 2. 编码 PNG(image/png)
    // 3. 写文件 %APPDATA%/shadow-worker/screenshots/2026-06-18/093015-Cursor.png
    // 4. 返回路径
}
```

---

## 6. 精度档位完整映射

```go
// internal/collector/movement.go

type PrecisionConfig struct {
    Thresh        uint8 // 像素差值阈值
    ChangeRatio   float64 // 变化像素占比阈值
    SampleMs      int    // 采样间隔
    IdleTimeoutS  int    // 静止超时
}

var presets = map[string]PrecisionConfig{
    "low":    {Thresh: 50, ChangeRatio: 0.005, SampleMs: 500, IdleTimeoutS: 15},
    "medium": {Thresh: 30, ChangeRatio: 0.002, SampleMs: 300, IdleTimeoutS: 10},
    "high":   {Thresh: 15, ChangeRatio: 0.001, SampleMs: 200, IdleTimeoutS: 5},
}
```

---

## 7. Go 调 Win32 的依赖

```
go.mod 加:
  golang.org/x/sys  (提供 windows.UTF16ToString 等)

syscall 调 user32/kernel32:
  - user32.dll: GetForegroundWindow, GetWindowTextW, GetWindowThreadProcessId, WindowFromPoint, GetWindowRect, BitBlt, PrintWindow, GetDC
  - kernel32.dll: OpenProcess, QueryFullProcessImageNameW, CloseHandle
  - gdi32.dll: CreateCompatibleDC, GetDIBits, SelectObject, DeleteObject
```

---

## 8. 移植参考(现有 ai-voice-tool 代码)

| 现有代码 | 可移植部分 |
|---------|-----------|
| `internal/floatbridge/screenshot.go` | Qt 侧截图触发流程(Go 触发 Qt 截图) |
| `floatwindow/screenshotwindow.cpp` | 全屏遮罩 + 拖拽选区 + 高亮(选窗口交互基础) |
| `Desktop/vga_detect.cpp` | 帧差算法逻辑(上面 FrameDiff 是它的 Go 移植) |

**注意**:新项目不沿用 floatbridge 的子进程架构,Go 直接调 Win32(不再经 Qt)。
