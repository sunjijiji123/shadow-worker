package collector

import (
	"sync"

	"golang.org/x/sys/windows"

	"shadow-worker/backend/internal/winapi"
)

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

		// 2. 取标题，跳过无标题窗口（通常是不可见的系统窗口或工具窗口）
		var titleBuf [512]uint16
		titleLen := winapi.GetWindowTextW(winapi.HWND(hwnd), &titleBuf[0], int32(len(titleBuf)))
		if titleLen == 0 {
			return 1
		}

		// 3. 取进程信息（失败则跳过该窗口）
		app, err := AppInfoFromHwnd(winapi.HWND(hwnd))
		if err != nil {
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
