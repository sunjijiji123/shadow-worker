package winapi

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess                = modkernel32.NewProc("OpenProcess")
	procCloseHandle                = modkernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = modkernel32.NewProc("QueryFullProcessImageNameW")
	procGetTickCount64             = modkernel32.NewProc("GetTickCount64")
)

// 进程访问权限常量。
const (
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
)

// Handle 是 Windows 句柄。
type Handle syscall.Handle

// OpenProcess 以指定权限打开进程。
func OpenProcess(desiredAccess uint32, inheritHandle bool, pid uint32) Handle {
	inherit := uintptr(0)
	if inheritHandle {
		inherit = 1
	}
	r, _, _ := procOpenProcess.Call(
		uintptr(desiredAccess),
		inherit,
		uintptr(pid),
	)
	return Handle(r)
}

// CloseHandle 关闭对象句柄。
func CloseHandle(h Handle) bool {
	r, _, _ := procCloseHandle.Call(uintptr(h))
	return r != 0
}

// QueryFullProcessImageNameW 查询进程完整路径。
func QueryFullProcessImageNameW(hProcess Handle, flags uint32, buf *uint16, size *uint32) bool {
	r, _, _ := procQueryFullProcessImageNameW.Call(
		uintptr(hProcess),
		uintptr(flags),
		uintptr(unsafe.Pointer(buf)),
		uintptr(unsafe.Pointer(size)),
	)
	return r != 0
}

// GetTickCount64 返回自系统启动以来的毫秒数(64 位,不会溢出)。
// 与 GetLastInputInfo 返回的 dwTime 同源,可直接相减得到空闲时长。
// (vendor 的 golang.org/x/sys/windows 未导出 GetTickCount64,故自行封装。)
func GetTickCount64() uint64 {
	r, _, _ := procGetTickCount64.Call()
	return uint64(r)
}
