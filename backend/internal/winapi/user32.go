package winapi

import (
	"syscall"
	"unsafe"
)

var (
	moduser32                   = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow     = moduser32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId = moduser32.NewProc("GetWindowThreadProcessId")
	procGetWindowTextW          = moduser32.NewProc("GetWindowTextW")
	procWindowFromPoint         = moduser32.NewProc("WindowFromPoint")
	procGetWindowRect           = moduser32.NewProc("GetWindowRect")
	procGetDC                   = moduser32.NewProc("GetDC")
	procReleaseDC               = moduser32.NewProc("ReleaseDC")
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
