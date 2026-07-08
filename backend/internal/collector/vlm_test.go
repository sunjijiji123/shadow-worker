package collector

import (
	"testing"
)

// TestSetInputActiveS 验证输入活跃阈值被正确写入 atomic 字段。
// isTypingActive 依赖此值兜底（<=0 时按 8s）。
func TestSetInputActiveS(t *testing.T) {
	v := &VLMCapturer{}
	if got := v.inputActiveS.Load(); got != 0 {
		t.Fatalf("零值 inputActiveS 应为 0，实际 %d", got)
	}
	v.SetInputActiveS(12)
	if got := v.inputActiveS.Load(); got != 12 {
		t.Fatalf("SetInputActiveS(12) 后应为 12，实际 %d", got)
	}
}

// TestIsTypingActiveDoesNotPanic 验证 isTypingActive 在零值 VLMCapturer 上可安全调用。
// LastInputInfo 是系统级只读 API，测试机空闲时返回 false（不阻断采集），
// 这里只断言"不 panic"，避免依赖测试机当前的空闲时长。
func TestIsTypingActiveDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("isTypingActive 不应 panic，实际 recover: %v", r)
		}
	}()
	v := &VLMCapturer{}
	v.SetInputActiveS(8)
	_ = v.isTypingActive() // 仅验证可调用，返回值依赖测试机空闲状态，不断言。
}
