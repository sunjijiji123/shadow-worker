package collector

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"log"
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
	bits, w, h, ok := captureWindowBGRA(hwnd)
	if !ok {
		return nil
	}
	return resizeNearestRGB(bits, int(w), int(h), CaptureTargetWidth, CaptureTargetHeight)
}

// captureWindowBGRA 截取指定窗口到 32-bit BGRA 像素缓冲。
// 封装 GetWindowRect→GetDC→CreateCompatibleDC→PrintWindow→GetDIBits 这段
// 所有窗口截图共用的 setup（CaptureWindow/CaptureWindowPNG/CaptureWindowThumbnail
// 三者原本各写一遍，抽出来去重）。走 captureMu 全局锁。失败返回 ok=false。
func captureWindowBGRA(hwnd winapi.HWND) (bits []byte, w, h int32, ok bool) {
	captureMu.Lock()
	defer captureMu.Unlock()

	var rect winapi.RECT
	if !winapi.GetWindowRect(hwnd, &rect) {
		log.Printf("[captureWindowBGRA] GetWindowRect 失败 hwnd=%d", hwnd)
		return nil, 0, 0, false
	}
	w = rect.Right - rect.Left
	h = rect.Bottom - rect.Top
	if w <= 0 || h <= 0 {
		log.Printf("[captureWindowBGRA] 尺寸无效 hwnd=%d w=%d h=%d", hwnd, w, h)
		return nil, 0, 0, false
	}

	hdc := winapi.GetDC(0)
	if hdc == 0 {
		log.Printf("[captureWindowBGRA] GetDC 失败 hwnd=%d", hwnd)
		return nil, 0, 0, false
	}
	defer winapi.ReleaseDC(0, hdc)

	memDC := winapi.CreateCompatibleDC(winapi.HDC(hdc))
	if memDC == 0 {
		log.Printf("[captureWindowBGRA] CreateCompatibleDC 失败 hwnd=%d", hwnd)
		return nil, 0, 0, false
	}
	defer winapi.DeleteDC(memDC)

	bmp := winapi.CreateCompatibleBitmap(winapi.HDC(hdc), w, h)
	if bmp == 0 {
		log.Printf("[captureWindowBGRA] CreateCompatibleBitmap 失败 hwnd=%d", hwnd)
		return nil, 0, 0, false
	}
	defer winapi.DeleteObject(winapi.Handle(bmp))

	old := winapi.SelectObject(memDC, winapi.Handle(bmp))
	defer winapi.SelectObject(memDC, old)

	if !winapi.PrintWindow(hwnd, syscall.Handle(memDC), winapi.PW_RENDERFULLCONTENT) {
		log.Printf("[captureWindowBGRA] PrintWindow 失败 hwnd=%d", hwnd)
		return nil, 0, 0, false
	}

	bits, ok = dibitsRGB(winapi.HDC(hdc), bmp, w, h)
	if !ok {
		log.Printf("[captureWindowBGRA] dibitsRGB 失败 hwnd=%d", hwnd)
	}
	return bits, w, h, ok
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

// resizeBGRA 把 32-bit BGRA 像素缓冲最邻近降采样，输出仍为 BGRA（4 字节/像素，
// 保留 alpha 字节）。与 resizeNearestRGB 同算法但不做 BGRA→RGB 转换，供
// CaptureWindowThumbnail 用（bitsToPNG 接受 BGRA 输入）。
func resizeBGRA(src []byte, srcW, srcH, dstW, dstH int) []byte {
	dst := make([]byte, dstW*dstH*4)
	for y := 0; y < dstH; y++ {
		sy := y * srcH / dstH
		for x := 0; x < dstW; x++ {
			sx := x * srcW / dstW
			srcIdx := (sy*srcW + sx) * 4
			dstIdx := (y*dstW + x) * 4
			if srcIdx+3 >= len(src) {
				continue
			}
			dst[dstIdx+0] = src[srcIdx+0]
			dst[dstIdx+1] = src[srcIdx+1]
			dst[dstIdx+2] = src[srcIdx+2]
			dst[dstIdx+3] = src[srcIdx+3]
		}
	}
	return dst
}

// resizeBGRAFitLetterbox 把任意宽高比的 BGRA 原图等比缩放进 dstW×dstH 框内并
// 居中，四周补深色边带（letterbox），输出尺寸恒为 dstW×dstH 的 BGRA。供缩略图
// 预览用：原始窗口可能不是 16:9，直接拉伸会变形，缩放后再裁剪又丢内容。
// 边带填 #18181B（与卡片背景 Theme.bg 一致），视觉上无缝融入卡片。
// 输出 BGRA（4 字节/像素），可直接喂 bitsToPNG。
func resizeBGRAFitLetterbox(src []byte, srcW, srcH, dstW, dstH int) []byte {
	dst := make([]byte, dstW*dstH*4)

	// 边带填深色 #18181B（BGRA 字节序：B=0x18,G=0x18,B=0x1B,A=0xFF）。
	for i := 0; i < dstW*dstH; i++ {
		p := i * 4
		dst[p+0] = 0x18
		dst[p+1] = 0x18
		dst[p+2] = 0x1B
		dst[p+3] = 0xFF
	}
	if srcW <= 0 || srcH <= 0 {
		return dst
	}

	// 等比缩放：取宽高两个缩放比的较小者，保证整框装进 dstW×dstH 不变形。
	scaleW := float64(dstW) / float64(srcW)
	scaleH := float64(dstH) / float64(srcH)
	scale := scaleW
	if scaleH < scaleW {
		scale = scaleH
	}
	fitW := int(float64(srcW) * scale)
	fitH := int(float64(srcH) * scale)
	if fitW < 1 {
		fitW = 1
	}
	if fitH < 1 {
		fitH = 1
	}
	offX := (dstW - fitW) / 2
	offY := (dstH - fitH) / 2

	// 最邻近采样原图到 fitW×fitH，再平移到边框中央 (offX, offY)。
	for y := 0; y < fitH; y++ {
		sy := y * srcH / fitH
		for x := 0; x < fitW; x++ {
			sx := x * srcW / fitW
			srcIdx := (sy*srcW + sx) * 4
			dstIdx := ((y+offY)*dstW + (x + offX)) * 4
			if srcIdx+3 >= len(src) || dstIdx+3 >= len(dst) {
				continue
			}
			dst[dstIdx+0] = src[srcIdx+0]
			dst[dstIdx+1] = src[srcIdx+1]
			dst[dstIdx+2] = src[srcIdx+2]
			dst[dstIdx+3] = src[srcIdx+3]
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
	bits, w, h, ok := captureWindowBGRA(hwnd)
	if !ok {
		return nil
	}
	return bitsToPNG(bits, w, h)
}

// CaptureWindowThumbnail 截取指定窗口并降采样为 PNG，供"添加采集应用"网格预览
// 懒加载用（经 GetWindowThumbnail RPC）。
//
// 适配策略（letterbox）：窗口原始宽高比各异（竖向窗口、超宽窗口），不能像
// movement 帧差那样暴力拉伸到固定 16:9（会变形）。这里把原图按比例缩到
// 目标框（CaptureTargetWidth×CaptureTargetHeight）内、居中放置，四周补深色
// (#18181B，与卡片背景一致)边带，输出固定尺寸 320×180 PNG。前端 Image 用
// PreserveAspectFit 即可完整显示原图、不变形、不裁剪。
//
// 走 captureMu 全局锁（与 movement/VLM 截图串行，避免对目标窗口叠加重绘）。
// 失败（窗口已关/hwnd 失效）返回 nil，由调用方返回空 png 交前端降级。
func CaptureWindowThumbnail(hwnd winapi.HWND) []byte {
	bits, w, h, ok := captureWindowBGRA(hwnd)
	if !ok {
		log.Printf("[CaptureWindowThumbnail] 失败 hwnd=%d", hwnd)
		return nil
	}
	// 纯色判空：部分窗口（系统辅助窗口、无客户区渲染的窗口）PrintWindow 能成功，
	// 但截到的是全黑/纯色画面（日志表现为 bytes≈600~1600 的极小 PNG）。这类
	// "截图成功但无内容"的窗口对预览无意义，判定为空返回 nil，让前端走首字母
	// 占位图（比纯色块信息量大）。在 letterbox 前用原始 bits 判，避免边带干扰。
	if isSolidColor(bits, int(w), int(h)) {
		log.Printf("[CaptureWindowThumbnail] 截图为纯色，判空 hwnd=%d", hwnd)
		return nil
	}
	thumb := resizeBGRAFitLetterbox(bits, int(w), int(h),
		CaptureTargetWidth, CaptureTargetHeight)
	return bitsToPNG(thumb, CaptureTargetWidth, CaptureTargetHeight)
}

// isSolidColor 检测 BGRA 像素缓冲是否整体只有一个颜色（全黑/纯色）。
// 用抽样 + 容差：采样若干像素，若与第一个像素的色差全在 tol 内即判定纯色。
// 用于缩略图预览：纯色截图（PrintWindow 截到的空白画面）等同于无内容。
func isSolidColor(bits []byte, w, h int) bool {
	if w <= 0 || h <= 0 || len(bits) < w*h*4 {
		return true // 数据不全，视为无内容
	}
	const tol = 8 // 色差容差，吸收压缩/抖动噪声
	// 抽样：取第一个像素为基准，网格采样约 400 个点（足够判定纯色）。
	stepX := w / 20
	stepY := h / 15
	if stepX < 1 {
		stepX = 1
	}
	if stepY < 1 {
		stepY = 1
	}
	b0 := bits[0]
	b1 := bits[1]
	b2 := bits[2]
	for y := 0; y < h; y += stepY {
		for x := 0; x < w; x += stepX {
			i := (y*w + x) * 4
			if i+2 >= len(bits) {
				continue
			}
			if abs(int(bits[i])-int(b0)) > tol ||
				abs(int(bits[i+1])-int(b1)) > tol ||
				abs(int(bits[i+2])-int(b2)) > tol {
				return false // 出现色差，不是纯色
			}
		}
	}
	return true
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
