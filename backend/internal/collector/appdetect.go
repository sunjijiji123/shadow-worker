// Package collector 实现行为采集引擎。
package collector

import (
	"fmt"
	"path/filepath"
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

	var pid uint32
	winapi.GetWindowThreadProcessId(hwnd, &pid)
	if pid == 0 {
		return App{}, fmt.Errorf("无法获取前台窗口进程 ID")
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

	return App{
		Path:        fullPath,
		Name:        filepath.Base(fullPath),
		WindowTitle: title,
		PID:         pid,
		HWND:        hwnd,
	}, nil
}
