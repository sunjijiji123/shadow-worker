package collector

import (
	"testing"
	"time"

	"shadow-worker/backend/internal/config"
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
		coll := NewCollector(nil, config.MovementConfig{Precision: name}, nil)
		// 验证新字段在所有 Preset 中都有合理非零值
		if coll.cfg.InputIdleS <= 0 {
			t.Errorf("Preset %s 的 InputIdleS 不应为 0", name)
		}
		if coll.cfg.DisplayIdleS <= 0 {
			t.Errorf("Preset %s 的 DisplayIdleS 不应为 0", name)
		}
		if coll.cfg.DisplayIdleS <= coll.cfg.InputIdleS {
			t.Errorf("Preset %s 的 DisplayIdleS(%d) 应大于 InputIdleS(%d)",
				name, coll.cfg.DisplayIdleS, coll.cfg.InputIdleS)
		}
	}
}

// TestNewCollectorConfigOverride 验证 config 字段非 0 时覆盖 Preset。
func TestNewCollectorConfigOverride(t *testing.T) {
	// 用 config 显式指定超时,应覆盖 medium Preset 的默认(15/90)。
	coll := NewCollector(nil, config.MovementConfig{
		Precision:    "medium",
		InputIdleS:   42,
		DisplayIdleS: 200,
	}, nil)
	if coll.cfg.InputIdleS != 42 {
		t.Errorf("InputIdleS 应被 config 覆盖为 42，实际 %d", coll.cfg.InputIdleS)
	}
	if coll.cfg.DisplayIdleS != 200 {
		t.Errorf("DisplayIdleS 应被 config 覆盖为 200，实际 %d", coll.cfg.DisplayIdleS)
	}
	// SampleIntervalMs=0 时应保留 Preset 的 300。
	if coll.cfg.SampleMs != 300 {
		t.Errorf("SampleIntervalMs 未指定时应保留 Preset 300，实际 %d", coll.cfg.SampleMs)
	}
}

// TestNewCollectorUnknownPrecision 验证未知 precision 回退 medium。
func TestNewCollectorUnknownPrecision(t *testing.T) {
	coll := NewCollector(nil, config.MovementConfig{Precision: "bogus"}, nil)
	if coll.cfg.InputIdleS != Presets["medium"].InputIdleS {
		t.Errorf("未知 precision 应回退 medium 的 InputIdleS=%d，实际 %d",
			Presets["medium"].InputIdleS, coll.cfg.InputIdleS)
	}
}

// TestInferState 三态判定纯函数单测。
func TestInferState(t *testing.T) {
	displayIdle := 90 * time.Second

	tests := []struct {
		name         string
		strong       bool
		sinceEngaged time.Duration
		want         string
	}{
		{"强信号→engaged(忽略时间)", true, 200 * time.Second, StateEngaged},
		{"强信号→engaged(刚活跃)", true, 1 * time.Second, StateEngaged},
		{"无信号+宽限期内→active", false, 30 * time.Second, StateActive},
		{"无信号+宽限期内(边界-1)→active", false, 89 * time.Second, StateActive},
		{"无信号+超宽限期→idle", false, 91 * time.Second, StateIdle},
		{"无信号+远超宽限期→idle", false, 600 * time.Second, StateIdle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferState(tt.strong, tt.sinceEngaged, displayIdle)
			if got != tt.want {
				t.Errorf("inferState(%v, %v, %v) = %q，want %q",
					tt.strong, tt.sinceEngaged, displayIdle, got, tt.want)
			}
		})
	}
}
