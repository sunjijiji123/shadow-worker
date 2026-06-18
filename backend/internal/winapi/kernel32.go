package winapi

import (
	"syscall"
	"unsafe"
)

var (
	modkernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess                 = modkernel32.NewProc("OpenProcess")
	procCloseHandle                 = modkernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW  = modkernel32.NewProc("QueryFullProcessImageNameW")
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
