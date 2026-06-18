package collector

import (
	"testing"
)

func TestFrameDiffIdentical(t *testing.T) {
	w, h := 320, 180
	frame := make([]byte, w*h*3)
	for i := range frame {
		frame[i] = byte(i % 256)
	}

	ratio, err := FrameDiff(frame, frame, w, h)
	if err != nil {
		t.Fatalf("FrameDiff 失败: %v", err)
	}
	if ratio != 0 {
		t.Fatalf("相同帧的变化比例应为 0，实际 %.6f", ratio)
	}
}

func TestFrameDiffAllChanged(t *testing.T) {
	w, h := 320, 180
	prev := make([]byte, w*h*3)
	curr := make([]byte, w*h*3)
	for i := range curr {
		curr[i] = 255
	}

	ratio, err := FrameDiff(prev, curr, w, h)
	if err != nil {
		t.Fatalf("FrameDiff 失败: %v", err)
	}
	// 扫描中间 85% 像素，全部变化
	expected := 0.85
	if ratio < expected-0.01 || ratio > expected+0.01 {
		t.Fatalf("全变帧的变化比例应接近 %.2f，实际 %.6f", expected, ratio)
	}
}

func TestFrameDiffSizeMismatch(t *testing.T) {
	_, err := FrameDiff([]byte{1, 2, 3}, []byte{1, 2, 3}, 10, 10)
	if err == nil {
		t.Fatal("尺寸不匹配时应返回错误")
	}
}

func TestPresets(t *testing.T) {
	for name := range Presets {
		_ = NewCollector(nil, name, nil)
	}
}
