package collector

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"sync"
	"syscall"
	"unsafe"

	"shadow-worker/backend/internal/winapi"
)

// CaptureTargetWidth / CaptureTargetHeight 是 movement 帧差用的降采样分辨率。
const (
	CaptureTargetWidth  = 320
	CaptureTargetHeight = 180
)

// captureMu 是所有截图函数共用的全局互斥锁。
// movement loop(每 300ms 截帧差图)和 vlm OnActivity(触发时截识别图)是两个
// 独立 goroutine，会并发对同一 HWND 调用 PrintWindow。GDI 层面线程安全，但两个
// PrintWindow 同时让目标窗口 UI 线程做合成重绘会让卡顿叠加（尤其 Electron 应用）。
// 串行化后最坏情况是截图排队等一拍，但 ticker 节流，下一个 tick 补上，不影响正确性，
// 且让卡顿"平摊"而非"叠加"，目标窗口感知更轻。
var captureMu sync.Mutex

// CaptureWindow 截取指定窗口并降采样为 RGB 字节（长度=320*180*3）。
// 失败返回 nil。
//
// 用 PrintWindow(PW_RENDERFULLCONTENT) 代替 GetDC+BitBlt：硬件加速窗口
// （Electron/CEF）用 BitBlt 只能拿到空白/加载态，导致帧差信号失效、
// state 误判。PrintWindow 对所有窗口类型都能拿到真实内容。
func CaptureWindow(hwnd winapi.HWND) []byte {
	captureMu.Lock()
	defer captureMu.Unlock()

	var rect winapi.RECT
	if !winapi.GetWindowRect(hwnd, &rect) {
		return nil
	}
	w := rect.Right - rect.Left
	h := rect.Bottom - rect.Top
	if w <= 0 || h <= 0 {
		return nil
	}

	hdc := winapi.GetDC(0)
	if hdc == 0 {
		return nil
	}
	defer winapi.ReleaseDC(0, hdc)

	memDC := winapi.CreateCompatibleDC(winapi.HDC(hdc))
	if memDC == 0 {
		return nil
	}
	defer winapi.DeleteDC(memDC)

	bmp := winapi.CreateCompatibleBitmap(winapi.HDC(hdc), w, h)
	if bmp == 0 {
		return nil
	}
	defer winapi.DeleteObject(winapi.Handle(bmp))

	old := winapi.SelectObject(memDC, winapi.Handle(bmp))
	defer winapi.SelectObject(memDC, old)

	if !winapi.PrintWindow(hwnd, syscall.Handle(memDC), winapi.PW_RENDERFULLCONTENT) {
		return nil
	}

	bits, ok := dibitsRGB(winapi.HDC(hdc), bmp, w, h)
	if !ok {
		return nil
	}

	return resizeNearestRGB(bits, int(w), int(h), CaptureTargetWidth, CaptureTargetHeight)
}

// resizeNearestRGB 把 32-bit BGRA 最邻近降采样为 RGB。
func resizeNearestRGB(src []byte, srcW, srcH, dstW, dstH int) []byte {
	dst := make([]byte, dstW*dstH*3)
	for y := 0; y < dstH; y++ {
		sy := y * srcH / dstH
		for x := 0; x < dstW; x++ {
			sx := x * srcW / dstW
			srcIdx := (sy*srcW + sx) * 4
			dstIdx := (y*dstW + x) * 3
			if srcIdx+2 >= len(src) {
				continue
			}
			// BGRA -> RGB
			dst[dstIdx+0] = src[srcIdx+2]
			dst[dstIdx+1] = src[srcIdx+1]
			dst[dstIdx+2] = src[srcIdx+0]
		}
	}
	return dst
}

// FrameDiff 比较两帧 RGB，返回变化像素比例。
func FrameDiff(prev, curr []byte, w, h int) (float64, error) {
	if len(prev) != len(curr) || len(prev) != w*h*3 {
		return 0, fmt.Errorf("帧尺寸不匹配: prev=%d curr=%d want=%d", len(prev), len(curr), w*h*3)
	}

	const thresh = 30
	start := w * h * 5 / 100
	end := w * h * 90 / 100
	if start < 0 {
		start = 0
	}
	if end > w*h {
		end = w * h
	}

	changed := 0
	for i := start; i < end; i++ {
		p := i * 3
		if abs(int(prev[p])-int(curr[p])) > thresh ||
			abs(int(prev[p+1])-int(curr[p+1])) > thresh ||
			abs(int(prev[p+2])-int(curr[p+2])) > thresh {
			changed++
		}
	}
	return float64(changed) / float64(w*h), nil
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// CaptureWindowPNG 截取指定窗口并返回 PNG 字节。
//
// 使用 PrintWindow(PW_RENDERFULLCONTENT) 而非 GetDC+BitBlt：后者对硬件加速/
// 合成渲染窗口（Electron/CEF 内壳的 IDE 如 VS Code、ZCode）只能拿到空白或
// 加载态画面。PrintWindow 让窗口把内容绘制到我们提供的 DC，对所有窗口类型都
// 有效（Windows 8.1+）。失败返回 nil。
func CaptureWindowPNG(hwnd winapi.HWND) []byte {
	captureMu.Lock()
	defer captureMu.Unlock()

	var rect winapi.RECT
	if !winapi.GetWindowRect(hwnd, &rect) {
		return nil
	}
	w := rect.Right - rect.Left
	h := rect.Bottom - rect.Top
	if w <= 0 || h <= 0 {
		return nil
	}

	hdc := winapi.GetDC(0)
	if hdc == 0 {
		return nil
	}
	defer winapi.ReleaseDC(0, hdc)

	memDC := winapi.CreateCompatibleDC(winapi.HDC(hdc))
	if memDC == 0 {
		return nil
	}
	defer winapi.DeleteDC(memDC)

	bmp := winapi.CreateCompatibleBitmap(winapi.HDC(hdc), w, h)
	if bmp == 0 {
		return nil
	}
	defer winapi.DeleteObject(winapi.Handle(bmp))

	old := winapi.SelectObject(memDC, winapi.Handle(bmp))
	defer winapi.SelectObject(memDC, old)

	// PrintWindow 把窗口内容绘制到 memDC。PW_RENDERFULLCONTENT 对硬件加速
	// 窗口必需；对普通窗口也兼容（相当于标准绘制）。
	if !winapi.PrintWindow(hwnd, syscall.Handle(memDC), winapi.PW_RENDERFULLCONTENT) {
		return nil
	}

	return dibitsToPNG(winapi.HDC(hdc), bmp, w, h)
}

// CaptureScreenPNG 截取整块虚拟屏幕（所有显示器并集）并返回 PNG 字节。
// 多显示器场景下虚拟屏原点可能为负；GetDC(0) 的 DC 覆盖整块虚拟屏，
// BitBlt 源坐标恒为 (0,0)（DC 已对齐到虚拟屏左上角）。失败返回 nil。
func CaptureScreenPNG() []byte {
	captureMu.Lock()
	defer captureMu.Unlock()

	w := winapi.GetSystemMetrics(winapi.SM_CXVIRTUALSCREEN)
	h := winapi.GetSystemMetrics(winapi.SM_CYVIRTUALSCREEN)
	if w <= 0 || h <= 0 {
		return nil
	}

	hdc := winapi.GetDC(0)
	if hdc == 0 {
		return nil
	}
	defer winapi.ReleaseDC(0, hdc)

	return capturePNGFromDC(winapi.HDC(hdc), 0, 0, w, h)
}

// capturePNGFromDC 从 srcDC 的 (x,y) 起，把 w×h 区域 BitBlt 到兼容内存 DC，
// GetDIBits 读出 32-bit BGRA，转 RGBA 后 png.Encode 返回。
// 失败返回 nil。调用方负责 srcDC 的 GetDC/ReleaseDC 配对。
func capturePNGFromDC(srcDC winapi.HDC, x, y, w, h int32) []byte {
	memDC := winapi.CreateCompatibleDC(srcDC)
	if memDC == 0 {
		return nil
	}
	defer winapi.DeleteDC(memDC)

	bmp := winapi.CreateCompatibleBitmap(srcDC, w, h)
	if bmp == 0 {
		return nil
	}
	defer winapi.DeleteObject(winapi.Handle(bmp))

	old := winapi.SelectObject(memDC, winapi.Handle(bmp))
	defer winapi.SelectObject(memDC, old)

	if !winapi.BitBlt(memDC, 0, 0, w, h, srcDC, x, y, winapi.SRCCOPY) {
		return nil
	}

	return dibitsToPNG(srcDC, bmp, w, h)
}

// dibitsToPNG 从已绘制好的位图 bmp 读出 32-bit BGRA 像素，转 RGBA PNG。
// srcDC 必须与 bmp 兼容（用于 GetDIBits 的格式查询）。失败返回 nil。
// 供 capturePNGFromDC(BitBlt) 和 CaptureWindowPNG(PrintWindow) 共用。
func dibitsToPNG(srcDC winapi.HDC, bmp winapi.HBITMAP, w, h int32) []byte {
	bits, ok := dibitsRGB(srcDC, bmp, w, h)
	if !ok {
		return nil
	}
	return bitsToPNG(bits, w, h)
}

// dibitsRGB 从已绘制好的位图 bmp 读出 32-bit BGRA 像素缓冲。
// 供需要原始像素的帧差路径（CaptureWindow → resizeNearestRGB）使用。
func dibitsRGB(srcDC winapi.HDC, bmp winapi.HBITMAP, w, h int32) ([]byte, bool) {
	bufSize := int(w) * int(h) * 4
	bits := make([]byte, bufSize)

	bmi := &winapi.BITMAPINFO{}
	bmi.BmiHeader.BiSize = uint32(unsafe.Sizeof(bmi.BmiHeader))
	bmi.BmiHeader.BiWidth = w
	bmi.BmiHeader.BiHeight = -h // 顶-down
	bmi.BmiHeader.BiPlanes = 1
	bmi.BmiHeader.BiBitCount = 32
	bmi.BmiHeader.BiCompression = 0
	bmi.BmiHeader.BiSizeImage = uint32(bufSize)

	lines := winapi.GetDIBits(srcDC, bmp, 0, uint32(h), unsafe.Pointer(&bits[0]), bmi, winapi.DIB_RGB_COLORS)
	if lines == 0 {
		return nil, false
	}
	return bits, true
}

// bitsToPNG 把 32-bit BGRA 像素缓冲转为 RGBA PNG。失败返回 nil。
func bitsToPNG(bits []byte, w, h int32) []byte {
	img := image.NewRGBA(image.Rect(0, 0, int(w), int(h)))
	for y := 0; y < int(h); y++ {
		for x := 0; x < int(w); x++ {
			src := (y*int(w) + x) * 4
			dst := img.PixOffset(x, y)
			img.Pix[dst+0] = bits[src+2] // R
			img.Pix[dst+1] = bits[src+1] // G
			img.Pix[dst+2] = bits[src+0] // B
			img.Pix[dst+3] = 0xFF
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}
