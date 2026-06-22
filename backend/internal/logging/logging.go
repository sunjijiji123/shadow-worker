// Package logging 配置全局 slog 日志：按天滚动文件 + 可选控制台输出。
//
// 日志文件：%APPDATA%/shadow-worker/logs/shadow-worker-YYYY-MM-DD.log
// 按天滚动：每天首条日志创建当天文件（不预先创建）。启动时清理超过
// retentionDays 的旧文件。
//
// 用法（main.go 启动早期调用一次）：
//
//	closer, err := logging.Init(cfg.Log)
//	if err != nil { ... }
//	defer closer()  // 确保缓冲刷盘
//
// 之后所有 slog.Default()/slog.Info() 等调用都会写入配置的输出。
package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 按天滚动的 Writer：首次写当天时打开当天文件，跨天自动切换。
// 同时支持叠加控制台输出（io.MultiWriter）。
type dailyFileWriter struct {
	mu       sync.Mutex
	logsDir  string // logs 目录绝对路径
	day      string // 当前文件对应的日期 "YYYY-MM-DD"
	file     *os.File
	console  io.Writer // 非 nil 时同时输出到控制台（stderr）
}

func (w *dailyFileWriter) ensureToday(now time.Time) error {
	today := now.Format("2006-01-02")
	if w.day == today && w.file != nil {
		return nil
	}
	// 跨天/首次：关闭旧文件，打开当天文件（O_APPEND，不覆盖）。
	if w.file != nil {
		w.file.Close()
	}
	name := fmt.Sprintf("shadow-worker-%s.log", today)
	path := filepath.Join(w.logsDir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败 %s: %w", path, err)
	}
	w.file = f
	w.day = today
	return nil
}

// Write 实现 io.Writer。slog 的 Handler 会把格式化后的字节写入这里。
// 加锁保证并发安全（slog 多 goroutine 调用）。
func (w *dailyFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ensureToday(time.Now()); err != nil {
		return 0, err
	}
	n, err := w.file.Write(p)
	if err != nil {
		return n, err
	}
	if w.console != nil {
		// 控制台写入失败不影响文件（忽略错误）。
		w.console.Write(p)
	}
	return n, nil
}

// Close 关闭当前日志文件。应在程序退出前调用（defer）确保缓冲刷盘。
func (w *dailyFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

// slogWriter 把标准库 log 包的输出桥接到 slog。
//
// 用法：log.SetOutput(&slogWriter{logger: ...})。这样 main.go 里保留的
// log.Fatalf/log.Printf（启动期致命错误等）也会进日志文件，而不是只到 stderr。
//
// 按行切分记录：log.Printf 一次 Write 可能含多行（含末尾换行），逐行作为
// 一条 Info 记录，避免多行挤在一条日志里。无并发保护——log 包自身在每次
// Write 时持锁，故 Write 不会并发。
type slogWriter struct {
	logger *slog.Logger
}

func (sw *slogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\r\n")
	if msg == "" {
		return len(p), nil
	}
	// 多行：逐行记一条（log.Printf 极少多行，但 Fatalf 的错误信息可能含换行）。
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			sw.logger.Info(line, "source", "stdlog")
		}
	}
	return len(p), nil
}

// parseLevel 把配置字符串解析为 slog.Level。默认 info。
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default: // "info" 或未知
		return slog.LevelInfo
	}
}

// Init 初始化全局日志：创建 logs 目录、清理过期日志、设置 slog.Default。
//
// 返回 closer，调用方应 defer 它（关文件刷盘）。
// 注意：必须在 config.Load() 之后、任何业务日志之前调用。
func Init(level string, console bool, retentionDays int) (func(), error) {
	// logs 目录与 config.yaml 同级：os.UserConfigDir()/shadow-worker/logs
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("获取配置目录失败: %w", err)
	}
	logsDir := filepath.Join(cfgDir, "shadow-worker", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建日志目录失败: %w", err)
	}

	// 启动时清理过期日志（失败不阻断启动）。
	if retentionDays > 0 {
		cleanupOldLogs(logsDir, retentionDays)
	}

	w := &dailyFileWriter{
		logsDir: logsDir,
		console: nil,
	}
	if console {
		w.console = os.Stderr
	}

	handler := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: parseLevel(level),
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// 让标准库 log 包（log.Printf/Fatalf 等）也走同一 handler，统一日志出口。
	// main.go 里保留的 log.Fatalf（启动期致命错误）经此桥接进文件，不丢到 stderr。
	// 实现：一个 io.Writer，Write 时把每行作为 slog.Info 记录（按行切分，
	// 因为 log.Printf 一次 Write 可能含多行）。
	log.SetOutput(&slogWriter{logger: logger})
	log.SetFlags(0) // slog 已带时间戳，去掉标准 log 的日期前缀

	slog.Info("日志已初始化",
		"dir", logsDir, "level", level,
		"console", console, "retention_days", retentionDays)

	return func() { _ = w.Close() }, nil
}

// cleanupOldLogs 删除 logs 目录下超过 retentionDays 天的 shadow-worker-*.log。
// 按文件 mtime 判断（比文件名日期更准，能处理跨天未关闭的情况）。
func cleanupOldLogs(logsDir string, retentionDays int) {
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return // 读不了就跳过，不阻断启动
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "shadow-worker-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(logsDir, name))
		}
	}
}
