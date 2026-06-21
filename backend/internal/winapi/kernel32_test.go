package winapi

import (
	"testing"
)

// TestGetTickCount64 是真实系统调用冒烟测试(需 Windows 会话)。
// 验证 GetTickCount64 返回合理值:系统已运行至少 1 秒(测试环境通常远超)。
func TestGetTickCount64(t *testing.T) {
	tick := GetTickCount64()
	if tick == 0 {
		t.Fatal("GetTickCount64 返回 0,预期为非零的系统运行时长")
	}
	// 至少应大于 1000ms(测试机不可能启动后 1 秒内跑测试)。
	if tick < 1000 {
		t.Fatalf("GetTickCount64=%d 异常偏小,预期 > 1000ms", tick)
	}
}
