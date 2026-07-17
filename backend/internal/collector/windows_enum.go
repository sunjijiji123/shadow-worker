package collector

import (
	"strings"
	"sync"

	"golang.org/x/sys/windows"

	"shadow-worker/backend/internal/winapi"
)

// desktopWindowClasses 是 Windows 桌面/任务栏的窗口类名黑名单。
// 这些窗口虽"无所有者 + 可见 + 有标题"，但不是用户工作目标，枚举可跟踪
// 应用时排除：
//   - Progman / WorkerW：桌面（explorer.exe 托管，标题常为 "Program Manager"）。
//     WorkerW 是绘制壁纸的叠加层，和 Progman 一起构成"桌面"。
//   - Shell_TrayWnd：任务栏。
//   - Shell_SecondaryTrayWnd：多显示器副任务栏。
//
// 注意：explorer.exe 还托管文件资源管理器窗口（类名 CabinetWClass），那个
// 不在黑名单——浏览项目文件对很多人是工作的一部分。按类名而非进程名排除，
// 精准去掉桌面、保留文件窗口。
var desktopWindowClasses = map[string]bool{
	"Progman":                true,
	"WorkerW":                true,
	"Shell_TrayWnd":          true,
	"Shell_SecondaryTrayWnd": true,
}

// systemProcessBlacklist 是系统/辅助进程黑名单（基于进程名，去 .exe 后缀的小写）。
// 这些进程的窗口不是用户工作目标，作为类名过滤的兜底：某些系统窗口类名不固定
// 或枚举时拿不到类名，用进程名兜底排除。
//   - explorer：explorer.exe 同时托管桌面（Progman，已由类名排除）、任务栏
//     （Shell_TrayWnd，已由类名排除）和文件资源管理器窗口（CabinetWClass）。
//     对工作时间跟踪场景，浏览文件资源管理器不算工作内容、是噪声，整体排除。
//     （如个别用户确需跟踪文件浏览，可后续把 explorer 改成只靠类名排除桌面。）
//   - SearchHost / StartMenuExperienceHost / TextInputHost / WindowsTerminal 等：
//     开始菜单/搜索/Cortana/输入法 UI。
var systemProcessBlacklist = map[string]bool{
	// Windows shell：桌面/任务栏/文件资源管理器（explorer.exe 全家）
	"explorer":                true,
	// 开始菜单 / 搜索 / Cortana（Win10/11 各代进程名不同，都列）
	"searchhost":               true,
	"startmenuexperiencehost":  true,
	"shellexperiencehost":      true,
	// 输入法 UI
	"textinputhost":            true,
	"searchui":                 true,
	// 通知/操作中心
	"notificationcenter":       true,
}

// isDesktopOrSystemWindow 判断窗口是否为桌面/系统 UI，不应作为可跟踪应用枚举。
// className 为窗口类名，procName 为进程名（去 .exe 后缀，小写）。
func isDesktopOrSystemWindow(className, procName string) bool {
	if className != "" && desktopWindowClasses[className] {
		return true
	}
	if procName != "" && systemProcessBlacklist[strings.ToLower(procName)] {
		return true
	}
	return false
}

// VisibleWindows 枚举当前所有可见且带标题的顶层窗口，返回其应用信息。
//
// 用 EnumWindows（标准库 golang.org/x/sys/windows 已暴露）遍历所有顶层窗口，
// 过滤掉不可见（IsWindowVisible=FALSE）和标题为空的窗口。每个窗口复用
// AppInfoFromHwnd 取 path/name/title/pid。枚举失败的窗口跳过，不阻断整体。
//
// 去重：同一进程路径可能对应多个窗口，调用方按需处理；这里返回全部可见窗口，
// 由客户端展示窗口标题供用户区分（如多个 Cursor 窗口）。
func VisibleWindows() []App {
	var (
		mu   sync.Mutex
		apps []App
	)

	cb := windows.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		// 1. 只保留可见窗口
		if !windows.IsWindowVisible(windows.HWND(hwnd)) {
			return 1 // 继续枚举
		}

		// 2. 无所有者 = 独立顶层应用窗口。对话框/工具窗口/弹出辅助窗口
		//（输入法候选框、托盘宿主的子窗、各种 helper）都有 owner，排除它们，
		// 列表只剩 Alt+Tab/任务栏会出现的那种"应用"。这是用户认知里的应用窗口
		// 与系统辅助窗口的本质区别。
		if winapi.GetWindow(winapi.HWND(hwnd), winapi.GW_OWNER) != 0 {
			return 1
		}

		// 3. 排除被 DWM 隐藏（cloaked）的窗口。UWP/现代应用的后台窗口、最小化
		// 隐藏窗口虽 IsWindowVisible=TRUE，但 DWM 标记为 cloaked，对用户不可见、
		// 不在任务栏显示，不该作为可跟踪应用枚举。查询失败保守不过滤（返回 false）。
		if winapi.DwmCloaked(winapi.HWND(hwnd)) {
			return 1
		}

		// 4. 取标题，跳过无标题窗口（通常是不可见的系统窗口或工具窗口）
		var titleBuf [512]uint16
		titleLen := winapi.GetWindowTextW(winapi.HWND(hwnd), &titleBuf[0], int32(len(titleBuf)))
		if titleLen == 0 {
			return 1
		}

		// 5. 取窗口类名（用于排除桌面/任务栏等系统 UI 窗口）。
		var classBuf [256]uint16
		classLen := winapi.GetClassNameW(winapi.HWND(hwnd), &classBuf[0], int32(len(classBuf)))
		className := windows.UTF16ToString(classBuf[:classLen])

		// 6. 取进程信息（失败则跳过该窗口）
		app, err := AppInfoFromHwnd(winapi.HWND(hwnd))
		if err != nil {
			return 1
		}

		// 7. 排除桌面/任务栏/系统 UI 窗口（Program Manager 桌面、开始菜单、
		// 输入法 UI 等）。这些不是用户工作目标。
		if isDesktopOrSystemWindow(className, app.Name) {
			return 1
		}

		mu.Lock()
		apps = append(apps, app)
		mu.Unlock()
		return 1 // 继续枚举
	})

	// EnumWindows 在所有桌面枚举顶层窗口。回调签名是 WNDENUMPROC。
	_ = windows.EnumWindows(cb, nil)

	return apps
}
