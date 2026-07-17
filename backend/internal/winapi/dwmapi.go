package winapi

import (
	"syscall"
	"unsafe"
)

// DWM (Desktop Window Manager) API 封装。
//
// 用途：枚举可跟踪应用时，用 DWMWA_CLOAKED 判断窗口是否被 DWM 隐藏（"伪装"
// 不可见）。UWP/现代应用、最小化到后台的应用窗口虽然 IsWindowVisible=TRUE，
// 但 DWM 会把它们标记为 cloaked（对用户不可见、不在 Alt+Tab/任务栏显示）。
// 这类窗口不应作为可跟踪应用枚举出来。

var (
	moddwmapi                 = syscall.NewLazyDLL("dwmapi.dll")
	procDwmGetWindowAttribute = moddwmapi.NewProc("DwmGetWindowAttribute")
)

// DWMWA_CLOAKED：查询窗口是否被 DWM 隐藏。输出 BOOL（0=未隐藏，非0=隐藏）。
const DWMWA_CLOAKED = 14

// DwmCloaked 返回窗口是否被 DWM 隐藏（cloaked）。
// cloaked 窗口 IsWindowVisible=TRUE 但对用户不可见（UWP 后台、最小化隐藏窗口
// 等）。枚举可跟踪应用时应排除这类窗口。查询失败时保守返回 false（不过滤）。
func DwmCloaked(hwnd HWND) bool {
	var cloaked int32
	r, _, _ := procDwmGetWindowAttribute.Call(
		uintptr(hwnd),
		uintptr(DWMWA_CLOAKED),
		uintptr(unsafe.Pointer(&cloaked)),
		unsafe.Sizeof(cloaked),
	)
	return r == 0 && cloaked != 0
}
