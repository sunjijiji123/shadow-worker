package asr

import (
	"log/slog"
	"sync/atomic"

	"shadow-worker/backend/internal/config"
)

// EngineHolder 持有一个可热替换的 ASR 引擎。
//
// 后台服务（VoiceServer / AsrServer）共享同一个 *EngineHolder。当用户在设置页
// 修改 ASR 配置并保存后，ConfigServer 调用 Rebuild 重建引擎并原子替换，
// 之后所有 Recognize 调用自动使用新引擎，无需重启后端进程。
//
// 并发安全：Get 用 atomic load，Rebuild 在替换前完成新引擎的完整构造
// （包括本地引擎的模型加载），失败时保留旧引擎不动。
type EngineHolder struct {
	p      atomic.Pointer[Engine]
	logger *slog.Logger
}

// NewEngineHolder 用初始引擎构造一个 holder。
// logger 为 nil 时回退到 slog.Default()，供 Rebuild 热重载日志使用。
func NewEngineHolder(e Engine, logger *slog.Logger) *EngineHolder {
	if logger == nil {
		logger = slog.Default()
	}
	h := &EngineHolder{logger: logger}
	h.p.Store(&e)
	return h
}

// Get 返回当前引擎。永不为 nil（除非从未调用过 NewEngineHolder）。
func (h *EngineHolder) Get() Engine {
	e := h.p.Load()
	if e == nil {
		return nil
	}
	return *e
}

// Rebuild 根据新配置重建引擎并原子替换。
//
// 重建失败（如模型路径无效、网络配置非法）时，返回 error 但**保留旧引擎**，
// 这样运行中的录音不会因为坏配置而中断。调用方应将 error 反馈给用户。
func (h *EngineHolder) Rebuild(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	newEngine, err := New(cfg, h.logger)
	if err != nil {
		// 严重故障：配置已保存但引擎没换成，用户录音会继续用旧引擎（可能已不匹配配置）。
		h.logger.Error("热重载失败，保留旧引擎", "err", err)
		return err
	}
	old := h.Get()
	h.p.Store(&newEngine)
	h.logger.Info("引擎已热重载", "from", engineNameSafe(old), "to", newEngine.Name())
	return nil
}

func engineNameSafe(e Engine) string {
	if e == nil {
		return "<none>"
	}
	return e.Name()
}
