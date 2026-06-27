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
	procCreateMutexW               = modkernel32.NewProc("CreateMutexW")
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

// CreateMutex 创建或打开一个命名互斥体。
// 返回 (handle, alreadyExists, error)。
// alreadyExists=true 表示 mutex 已被其他进程创建（即已有实例在跑）。
//
// 用 Local\ 前缀（非 Global\）：Global\ 需要 SeCreateGlobalPrivilege，
// 非管理员用户会 ACCESS_DENIED。桌面应用用 Local\ 即可——每个会话独立，
// 不同 RDP 用户各跑一份。进程退出时内核自动回收 mutex，不会死锁。
func CreateMutex(name string) (Handle, bool, error) {
	n, _ := syscall.UTF16PtrFromString(name)
	r, _, e := procCreateMutexW.Call(
		0, // lpMutexAttributes = NULL
		0, // bInitialOwner = FALSE
		uintptr(unsafe.Pointer(n)),
	)
	if r == 0 {
		return 0, false, e
	}
	exists := e == syscall.ERROR_ALREADY_EXISTS
	return Handle(r), exists, nil
}

// ReleaseMutex 关闭互斥体句柄。
// 实际上进程退出时内核会自动回收，defer 调用是良好习惯。
func ReleaseMutex(h Handle) bool {
	r, _, _ := procCloseHandle.Call(uintptr(h))
	return r != 0
}
