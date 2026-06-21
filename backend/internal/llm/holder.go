package llm

import (
	"log"
	"sync/atomic"

	"shadow-worker/backend/internal/config"
)

// EngineHolder 持有一个可热替换的润色引擎（可持有 nil，表示润色未启用）。
//
// 后台服务（VoiceServer）共享同一个 *EngineHolder。当用户在设置页修改 LLM
// 配置并保存后，ConfigServer 调用 Rebuild 重建引擎并原子替换，之后所有
// Polish 调用自动使用新引擎，无需重启后端进程。
//
// 并发安全：Get 用 atomic load，Rebuild 在替换前完成新引擎构造，失败时
// 保留旧引擎不动（与 asr.EngineHolder 语义一致）。
type EngineHolder struct {
	p atomic.Pointer[Engine]
}

// NewEngineHolder 用初始引擎构造一个 holder。e 可为 nil（润色未启用）。
func NewEngineHolder(e Engine) *EngineHolder {
	h := &EngineHolder{}
	h.p.Store(&e)
	return h
}

// Get 返回当前引擎，可能为 nil（润色未启用）。
func (h *EngineHolder) Get() Engine {
	e := h.p.Load()
	if e == nil {
		return nil
	}
	return *e
}

// Rebuild 根据新配置重建引擎并原子替换。
//
// 只要配置了有效 provider 就创建引擎（手动润色可用），不受"自动润色"开关
// （cfg.LLM.Enabled）影响——后者只控制识别后是否自动触发。
// 重建失败（如 provider 找不到）时返回 error 但保留旧引擎，运行中的请求
// 不会因坏配置中断。调用方应将 error 反馈给用户。
func (h *EngineHolder) Rebuild(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	newEngine, err := New(cfg)
	if err != nil {
		log.Printf("[llm] holder rebuild failed, keeping old engine: %v", err)
		return err
	}
	old := h.Get()
	h.p.Store(&newEngine)
	log.Printf("[llm] holder rebuilt: %s -> %s", engineNameSafe(old), engineNameSafe(newEngine))
	return nil
}

func engineNameSafe(e Engine) string {
	if e == nil {
		return "<disabled>"
	}
	return e.Name()
}
