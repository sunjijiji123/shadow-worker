package winapi

import (
	"syscall"
	"unsafe"
)

var (
	moduser32                    = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow      = moduser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = moduser32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW           = moduser32.NewProc("GetWindowTextW")
	procWindowFromPoint          = moduser32.NewProc("WindowFromPoint")
	procGetWindowRect            = moduser32.NewProc("GetWindowRect")
	procGetDC                    = moduser32.NewProc("GetDC")
	procReleaseDC                = moduser32.NewProc("ReleaseDC")
	procGetLastInputInfo         = moduser32.NewProc("GetLastInputInfo")
	procGetSystemMetrics         = moduser32.NewProc("GetSystemMetrics")
)

// 虚拟屏幕（virtual screen）= 所有显示器的并集，原点在主显示器的左上角，
// 当副屏位于主屏左侧/上方时坐标可能为负。GetDC(0) 返回覆盖整块虚拟屏的 DC。
const (
	SM_XVIRTUALSCREEN  = 76 // 虚拟屏左上角 x（通常 ≤ 0）
	SM_YVIRTUALSCREEN  = 77 // 虚拟屏左上角 y（通常 ≤ 0）
	SM_CXVIRTUALSCREEN = 78 // 虚拟屏总宽（所有显示器宽之和，含间隙）
	SM_CYVIRTUALSCREEN = 79 // 虚拟屏总高
)

// HWND 是窗口句柄。
type HWND syscall.Handle

// POINT 是屏幕坐标点。
type POINT struct {
	X, Y int32
}

// RECT 是矩形区域。
type RECT struct {
	Left, Top, Right, Bottom int32
}

// LASTINPUTINFO 用于 GetLastInputInfo,记录系统最后一次输入事件的时间戳。
// DwTime 单位为毫秒,与 GetTickCount64 同源(自系统启动起算),二者可相减。
type LASTINPUTINFO struct {
	CbSize uint32
	DwTime uint32
}

// GetForegroundWindow 返回当前前台窗口句柄。
func GetForegroundWindow() HWND {
	r, _, _ := procGetForegroundWindow.Call()
	return HWND(r)
}

// GetWindowThreadProcessId 返回窗口所属线程，并写入进程 ID。
func GetWindowThreadProcessId(hwnd HWND, pid *uint32) uint32 {
	r, _, _ := procGetWindowThreadProcessId.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(pid)),
	)
	return uint32(r)
}

// GetWindowTextW 获取窗口标题。
func GetWindowTextW(hwnd HWND, buf *uint16, maxCount int32) int32 {
	r, _, _ := procGetWindowTextW.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(buf)),
		uintptr(maxCount),
	)
	return int32(r)
}

// WindowFromPoint 返回指定屏幕坐标下的窗口句柄。
func WindowFromPoint(pt POINT) HWND {
	r, _, _ := procWindowFromPoint.Call(uintptr(unsafe.Pointer(&pt)))
	return HWND(r)
}

// GetWindowRect 返回窗口在屏幕坐标下的矩形区域。
func GetWindowRect(hwnd HWND, rect *RECT) bool {
	r, _, _ := procGetWindowRect.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(rect)),
	)
	return r != 0
}

// GetDC 返回窗口设备上下文。
func GetDC(hwnd HWND) syscall.Handle {
	r, _, _ := procGetDC.Call(uintptr(hwnd))
	return syscall.Handle(r)
}

// ReleaseDC 释放设备上下文。
func ReleaseDC(hwnd HWND, hdc syscall.Handle) int32 {
	r, _, _ := procReleaseDC.Call(uintptr(hwnd), uintptr(hdc))
	return int32(r)
}

// GetSystemMetrics 查询系统度量/配置。用于获取虚拟屏（多显示器并集）的坐标和尺寸。
// nIndex 为 SM_* 常量；失败或未支持的索引返回 0。
func GetSystemMetrics(nIndex int32) int32 {
	r, _, _ := procGetSystemMetrics.Call(uintptr(nIndex))
	return int32(r)
}

// LastInputTick 返回系统最后一次键鼠输入事件的 tick(毫秒,自系统启动起算)。
// 失败(API 调用返回 0 或 DwTime 为 0)时 ok=false。
// 用法:与 GetTickCount64() 相减得到空闲毫秒数。
// 注意:这是系统级信号,不区分输入发给了哪个窗口;配合"前台=白名单"判定即可。
func LastInputTick() (tick uint32, ok bool) {
	info := LASTINPUTINFO{CbSize: uint32(unsafe.Sizeof(LASTINPUTINFO{}))}
	r, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 || info.DwTime == 0 {
		return 0, false
	}
	return info.DwTime, true
}
