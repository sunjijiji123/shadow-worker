package collector

import (
	"log/slog"
	"sync/atomic"

	"shadow-worker/backend/internal/config"
	"shadow-worker/backend/internal/storage"
)

// VLMHolder 用原子指针持有当前 VLMCapturer，支持配置变更后热重载。
//
// 与 ASR/LLM 的 EngineHolder 不同：VLMCapturer 有后台 goroutine（定时 loop /
// on_demand 消费 loop）和 Stop 生命周期，热重载需整体重建实例并管理协程启停，
// 不是纯指针替换。
//
// 线程安全：p 是 atomic.Pointer，Get/Rebuild 可跨 goroutine 并发调用。
type VLMHolder struct {
	p atomic.Pointer[VLMCapturer]
}

// NewVLMHolder 创建 holder。允许传入 nil（VLM 未配置 / off 模式），
// 此时 Get 返回 nil，Collector 回调时跳过。
func NewVLMHolder(c *VLMCapturer) *VLMHolder {
	h := &VLMHolder{}
	if c != nil {
		h.p.Store(c)
	}
	return h
}

// Get 返回当前 capturer；未启用时返回 nil。
// Collector.loop 和 CollectionServer.TriggerVLM 都通过它动态获取当前实例，
// 保证热重载后用上新 capturer。
func (h *VLMHolder) Get() *VLMCapturer {
	return h.p.Load()
}

// Rebuild 按新配置重建 VLMCapturer。
//
// 重建策略（整体重建，先启新后停旧）：
//  1. mode=off/"" 或 screen+on_demand 非法组合 → 构造 nil，停旧实例，Store nil。不算错误。
//  2. NewVLMCapturer 失败 → 记 log，保留旧实例，返回 error（配置已落盘，旧实例继续服务）。
//  3. 成功 → new.Start() → Store → old.Stop()（若有）。先启新后停旧，保证采集不中断。
//
// 注：old.Stop() 只关 stopCh 停 goroutine，不强杀正在跑的 in-flight Trigger
// （它持有 db 引用，跑完落库；这是热重载时序上可接受的行为）。
func (h *VLMHolder) Rebuild(cfg config.VLMConfig, db *storage.DB, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	// 非法组合兜底：整屏无活跃窗口概念，on_demand 无触发源。
	if cfg.Mode == "on_demand" && cfg.CaptureRange == "screen" {
		logger.Warn("screen 模式不支持 on_demand，降级为关闭",
			"mode", cfg.Mode, "range", cfg.CaptureRange)
		h.stopAndStore(nil)
		return nil
	}

	// off / 空 → 关闭。
	if cfg.Mode == "off" || cfg.Mode == "" {
		h.stopAndStore(nil)
		return nil
	}

	newCap, err := NewVLMCapturer(cfg, db, logger)
	if err != nil {
		// 严重故障：配置已保存但采集器没换成，保留旧 capturer（与 asr/llm holder 一致）。
		logger.Error("热重载失败，保留旧 capturer", "err", err)
		return err
	}

	// 先启新后停旧：保证热重载瞬间不中断采集。
	newCap.Start()
	h.stopAndStore(newCap)
	logger.Info("采集器已热重载",
		"mode", cfg.Mode, "range", cfg.CaptureRange, "engine", newCap.EngineName())
	return nil
}

// stopAndStore 是 Rebuild 的内部辅助：原子替换为新实例（可为 nil），
// 并停止被替换的旧实例（若有）。
func (h *VLMHolder) stopAndStore(newCap *VLMCapturer) {
	old := h.p.Swap(newCap)
	if old != nil {
		old.Stop()
	}
}
