// Package collector 实现行为采集引擎。
package collector

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"

	"shadow-worker/backend/internal/winapi"
)

// App 描述当前前台应用信息。
type App struct {
	Path        string // 完整进程路径，如 C:\Users\...\Cursor.exe
	Name        string // 进程名，如 Cursor.exe
	WindowTitle string // 窗口标题
	PID         uint32 // 进程 ID
	HWND        winapi.HWND
}

// ForegroundApp 返回当前前台窗口对应的应用信息。
// 失败时返回 error，上层应跳过本次采样。
func ForegroundApp() (App, error) {
	hwnd := winapi.GetForegroundWindow()
	if hwnd == 0 {
		return App{}, fmt.Errorf("没有前台窗口")
	}
	return AppInfoFromHwnd(hwnd)
}

// AppInfoFromHwnd 取任意顶层窗口句柄对应的应用信息（path/name/title/pid）。
// 与 ForegroundApp 共用同一段 OpenProcess + QueryFullProcessImageNameW 逻辑，
// 供枚举窗口（ListWindows）复用。用 PROCESS_QUERY_LIMITED_INFORMATION，无需管理员权限。
func AppInfoFromHwnd(hwnd winapi.HWND) (App, error) {
	var pid uint32
	winapi.GetWindowThreadProcessId(hwnd, &pid)
	if pid == 0 {
		return App{}, fmt.Errorf("无法获取窗口 %d 的进程 ID", hwnd)
	}

	hProc := winapi.OpenProcess(winapi.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if hProc == 0 {
		return App{}, fmt.Errorf("无法打开进程 %d", pid)
	}
	defer winapi.CloseHandle(hProc)

	var pathBuf [syscall.MAX_PATH]uint16
	size := uint32(len(pathBuf))
	if !winapi.QueryFullProcessImageNameW(hProc, 0, &pathBuf[0], &size) {
		return App{}, fmt.Errorf("无法查询进程 %d 路径", pid)
	}
	fullPath := windows.UTF16ToString(pathBuf[:size])

	var titleBuf [512]uint16
	titleLen := winapi.GetWindowTextW(hwnd, &titleBuf[0], int32(len(titleBuf)))
	title := windows.UTF16ToString(titleBuf[:titleLen])

	// Name 去掉 .exe 后缀，得到干净的进程名（如 Cursor 而非 Cursor.exe）。
	// collector 写入 activity_segments 用此 name，概览/时间轴聚合后显示一致，
	// 与白名单表 app_categories.name（addApp 时已清洗）对齐。
	base := filepath.Base(fullPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))

	return App{
		Path:        fullPath,
		Name:        name,
		WindowTitle: title,
		PID:         pid,
		HWND:        hwnd,
	}, nil
}
