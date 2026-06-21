package winapi

import (
	"testing"
)

// TestLastInputTick 是真实系统调用冒烟测试(需 Windows 会话)。
// 验证 LastInputTick 能拿到与 GetTickCount64 同源的时间戳,且二者差值合理。
// 注意:测试运行瞬间几乎必然有人机交互(测试进程本身就是被触发的),
// 故 lastInput 应接近 now,差值不会太大(给 5 分钟余量)。
func TestLastInputTick(t *testing.T) {
	lastInput, ok := LastInputTick()
	if !ok {
		t.Fatal("LastInputTick 返回 ok=false,预期在 Windows 会话中应成功")
	}
	if lastInput == 0 {
		t.Fatal("LastInputTick 返回 tick=0,异常")
	}

	now := GetTickCount64()
	// lastInput 是 uint32(ms),GetTickCount64 是 uint64(ms)。两者同源。
	// lastInput 不应超过 now(输入不可能发生在"未来")。
	if uint64(lastInput) > now {
		t.Fatalf("lastInput=%d 大于 now=%d,异常(输入不应发生在未来)", lastInput, now)
	}
	// 差值应在合理范围:测试运行时系统刚有过交互(至少启动测试的人)。
	// 给 5 分钟(300000ms)余量,排除 CI 无人值守的极端情况。
	idleMs := now - uint64(lastInput)
	if idleMs > 5*60*1000 {
		t.Fatalf("空闲 %dms 异常偏大,预期近期有人机交互", idleMs)
	}
	t.Logf("LastInputTick=%d, GetTickCount64=%d, idleMs=%d", lastInput, now, idleMs)
}
