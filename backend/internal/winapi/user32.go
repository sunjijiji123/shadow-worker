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
	procPrintWindow              = moduser32.NewProc("PrintWindow")
	procOpenInputDesktop          = moduser32.NewProc("OpenInputDesktop")
	procCloseDesktop              = moduser32.NewProc("CloseDesktop")
	procGetUserObjectInformationW = moduser32.NewProc("GetUserObjectInformationW")
	procGetWindow                = moduser32.NewProc("GetWindow")
	procGetClassNameW             = moduser32.NewProc("GetClassNameW")
)

// GetWindow 关系常量。
const (
	GW_HWNDFIRST = 0
	GW_HWNDLAST  = 1
	GW_HWNDNEXT  = 2
	GW_HWNDPREV  = 3
	GW_OWNER     = 4
	GW_CHILD     = 5
)

// 桌面访问与 UOI 常量。
const (
	DESKTOP_READOBJECTS = 0x0001
	UOI_NAME            = 2
)

// PrintWindow 标志位。
const (
	// PW_CLIENTONLY 仅截客户区。对很多应用客户区 DC 为空，故不单独使用。
	PW_CLIENTONLY = 0x00000001
	// PW_RENDERFULLCONTENT 让硬件加速/合成渲染（Electron/CEF/Chromium 内壳的
	// IDE 如 VS Code、ZCode）也能正确截取。GetDC+BitBlt 对这类窗口只能拿到
	// 空白/加载态画面，必须用 PrintWindow 配此标志。Windows 8.1+ 支持。
	PW_RENDERFULLCONTENT = 0x00000002
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

// GetClassNameW 获取窗口的类名（如 "Progman"、"CabinetWClass"）。
// 类名是窗口类型标识，比进程名更精准：explorer.exe 同时托管桌面（Progman）
// 和文件资源管理器（CabinetWClass），按类名可只排除桌面、保留文件窗口。
// 返回写入 buf 的字符数（不含结尾 \0），失败返回 0。
func GetClassNameW(hwnd HWND, buf *uint16, maxCount int32) int32 {
	r, _, _ := procGetClassNameW.Call(
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

// GetWindow 按关系常量（GW_OWNER / GW_CHILD 等）返回与 hwnd 相关的窗口句柄。
// GW_OWNER==0 表示 hwnd 无所有者（即独立顶层应用窗口，而非对话框/工具窗口
// 这类有 owner 的辅助窗口）。用于"枚举可跟踪应用"时区分应用窗口与辅助窗口。
func GetWindow(hwnd HWND, cmd uint32) HWND {
	r, _, _ := procGetWindow.Call(uintptr(hwnd), uintptr(cmd))
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

// PrintWindow 把窗口可视内容绘制到指定 DC。flags 为 PW_* 标志组合。
// 对硬件加速/合成渲染窗口（Electron/CEF 等），必须用 PW_RENDERFULLCONTENT
// 才能截到真实内容；GetDC+BitBlt 对这类窗口只能拿到空白/加载态。
// 成功返回 true。
func PrintWindow(hwnd HWND, hdc syscall.Handle, flags uint32) bool {
	r, _, _ := procPrintWindow.Call(uintptr(hwnd), uintptr(hdc), uintptr(flags))
	return r != 0
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

// IsWorkstationLocked 检测当前工作站是否处于锁屏状态。
// 原理：OpenInputDesktop 打开当前输入桌面；非锁屏时桌面名称为 "Default"，
// 锁屏/登录桌面时为 "Winlogon" 等其它名称。失败时保守返回 false（认为未锁屏）。
func IsWorkstationLocked() bool {
	h, _, _ := procOpenInputDesktop.Call(0, 0, uintptr(DESKTOP_READOBJECTS))
	if h == 0 {
		return false
	}
	defer procCloseDesktop.Call(h)

	var name [256]uint16
	var needed uint32
	r, _, _ := procGetUserObjectInformationW.Call(
		h,
		uintptr(UOI_NAME),
		uintptr(unsafe.Pointer(&name[0])),
		uintptr(len(name)*2),
		uintptr(unsafe.Pointer(&needed)),
	)
	if r == 0 {
		return false
	}
	return syscall.UTF16ToString(name[:]) != "Default"
}
