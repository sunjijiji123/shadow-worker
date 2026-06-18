package winapi

import (
	"syscall"
	"unsafe"
)

var (
	modgdi32                  = syscall.NewLazyDLL("gdi32.dll")
	procCreateCompatibleDC    = modgdi32.NewProc("CreateCompatibleDC")
	procDeleteDC              = modgdi32.NewProc("DeleteDC")
	procCreateCompatibleBitmap = modgdi32.NewProc("CreateCompatibleBitmap")
	procDeleteObject          = modgdi32.NewProc("DeleteObject")
	procSelectObject          = modgdi32.NewProc("SelectObject")
	procBitBlt                = modgdi32.NewProc("BitBlt")
	procGetDIBits             = modgdi32.NewProc("GetDIBits")
)

// HDC 是设备上下文句柄。
type HDC Handle

// HBITMAP 是位图句柄。
type HBITMAP Handle

// BITMAPINFOHEADER 用于 GetDIBits。
type BITMAPINFOHEADER struct {
	BiSize          uint32
	BiWidth         int32
	BiHeight        int32
	BiPlanes        uint16
	BiBitCount      uint16
	BiCompression   uint32
	BiSizeImage     uint32
	BiXPelsPerMeter int32
	BiYPelsPerMeter int32
	BiClrUsed       uint32
	BiClrImportant  uint32
}

// BITMAPINFO 用于 GetDIBits。
type BITMAPINFO struct {
	BmiHeader BITMAPINFOHEADER
	BmiColors [1]uint32
}

// RGBQUAD 是 RGB 颜色值。
type RGBQUAD struct {
	Blue     byte
	Green    byte
	Red      byte
	Reserved byte
}

// CreateCompatibleDC 创建兼容内存设备上下文。
func CreateCompatibleDC(hdc HDC) HDC {
	r, _, _ := procCreateCompatibleDC.Call(uintptr(hdc))
	return HDC(r)
}

// DeleteDC 删除设备上下文。
func DeleteDC(hdc HDC) bool {
	r, _, _ := procDeleteDC.Call(uintptr(hdc))
	return r != 0
}

// CreateCompatibleBitmap 创建兼容位图。
func CreateCompatibleBitmap(hdc HDC, width, height int32) HBITMAP {
	r, _, _ := procCreateCompatibleBitmap.Call(
		uintptr(hdc),
		uintptr(width),
		uintptr(height),
	)
	return HBITMAP(r)
}

// DeleteObject 删除 GDI 对象。
func DeleteObject(h Handle) bool {
	r, _, _ := procDeleteObject.Call(uintptr(h))
	return r != 0
}

// SelectObject 选入 GDI 对象。
func SelectObject(hdc HDC, h Handle) Handle {
	r, _, _ := procSelectObject.Call(uintptr(hdc), uintptr(h))
	return Handle(r)
}

// BitBlt 位块传输。
func BitBlt(dst HDC, dx, dy, width, height int32, src HDC, sx, sy int32, rop uint32) bool {
	r, _, _ := procBitBlt.Call(
		uintptr(dst),
		uintptr(dx), uintptr(dy),
		uintptr(width), uintptr(height),
		uintptr(src),
		uintptr(sx), uintptr(sy),
		uintptr(rop),
	)
	return r != 0
}

// GetDIBits 获取位图像素。
func GetDIBits(hdc HDC, hbmp HBITMAP, startScan, numScans uint32, bits unsafe.Pointer, bmi *BITMAPINFO, usage uint32) int32 {
	r, _, _ := procGetDIBits.Call(
		uintptr(hdc),
		uintptr(hbmp),
		uintptr(startScan),
		uintptr(numScans),
		uintptr(bits),
		uintptr(unsafe.Pointer(bmi)),
		uintptr(usage),
	)
	return int32(r)
}

// SRCCOPY 光栅操作码。
const SRCCOPY = 0x00CC0020

// DIB_RGB_COLORS 颜色表类型。
const DIB_RGB_COLORS = 0
