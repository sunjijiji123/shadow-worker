// Package httputil 提供 HTTP 客户端重试工具。
//
// 适用场景：云引擎（VLM/LLM/ASR）的 API 请求对瞬时失败（429 限流、5xx、网络
// 抖动、超时）按指数退避自动重试，避免一次 429 就丢弃整次采集/识别。
//
// 设计要点：
//   - reqBuilder 是闭包而非预构建 request —— http.Request 的 body 是 Reader，
//     读过一次就空了，重试必须重建。闭包捕获已序列化好的 []byte，重建只需
//     bytes.NewReader(capturedBytes)，不重复序列化。
//   - 只重试 HTTP 层失败（网络/状态码）。业务层失败（200 但 choices 为空）
//     不在此重试 —— 那是模型/逻辑问题，重试也是同样结果，交给调用方判定。
//   - 退避期间监听 ctx.Done()，保证采集停止 / 用户取消时立即退出。
package httputil

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"syscall"
	"time"
)

// 重试参数。
//   - DefaultMaxRetries：历史默认值，maxRetries ≤ 0 时回退到此（向后兼容旧行为）。
//   - baseBackoff / jitterFactor：指数退避 + 抖动，固定不变（非业务配置）。
const (
	DefaultMaxRetries = 3               // 默认重试次数（共 4 次请求：1 次初始 + 3 次重试）
	baseBackoff       = 1 * time.Second // 指数退避基数：attempt=0→1s, 1→2s, 2→4s
	jitterFactor      = 0.2             // ±20% 抖动，防限流风暴同步重试
)

// retryableStatusCodes 是值得重试的 HTTP 状态码：
//   - 429 Too Many Requests（限流，配合 Retry-After 头）
//   - 5xx 服务端错误（502 Bad Gateway / 503 Service Unavailable / 504 Gateway Timeout）
func retryableStatus(code int) bool {
	return code == 429 || code >= 500
}

// DoWithRetry 发送 HTTP 请求，对瞬时失败按指数退避重试。
//
// 参数：
//   - ctx：请求上下文。取消时立即退出重试循环（不睡死）。
//   - client：HTTP 客户端（含超时配置）。
//   - reqBuilder：构造请求的闭包。每次重试会调用它重建 request（body reader 一次性，
//     必须重建）。闭包应捕获已序列化好的 body bytes，重建时 bytes.NewReader 即可。
//   - logger：可选，nil 时用 slog.Default()。重试日志统一前缀 [httputil]。
//   - maxRetries：最多重试次数（不含首次请求）。≤0 时回退到 DefaultMaxRetries。
//     由各引擎从 provider 的 retry_count 透传；允许调用方按需配置。
//
// 返回：
//   - 成功（2xx/3xx/不可重试的 4xx）：返回 response + nil error。调用方判业务成功。
//   - 耗尽重试后仍失败：返回最后一次 response（可能 nil，如纯网络错误）+ error。
//
// 调用方负责关闭 resp.Body、读 body、判定业务成功（如 200 但 choices 为空不算重试范畴）。
func DoWithRetry(ctx context.Context, client *http.Client, reqBuilder func() (*http.Request, error), logger *slog.Logger, maxRetries int) (*http.Response, error) {
	if logger == nil {
		logger = slog.Default()
	}
	// maxRetries ≤ 0（含 0 / 未配置的零值）→ 回退默认，保持旧行为。
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}

	for attempt := 0; ; attempt++ {
		// 每次迭代开头检查 ctx：用户取消 / 采集停止时立即退出。
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req, err := reqBuilder()
		if err != nil {
			// 构造请求失败是逻辑错误（非网络瞬时），不重试。
			return nil, fmt.Errorf("构造请求失败: %w", err)
		}

		resp, doErr := client.Do(req)

		// 情况 1：网络错误（client.Do 返回 error）。
		if doErr != nil {
			retry := retryableError(doErr)
			logger.Info("请求失败",
				"attempt", attempt+1, "retryable", retry, "err", doErr)
			if !retry || attempt >= maxRetries {
				return nil, fmt.Errorf("请求失败（重试 %d 次）: %w", attempt, doErr)
			}
			if !sleep(ctx, backoff(attempt)) {
				return nil, ctx.Err()
			}
			continue
		}

		// 情况 2：拿到 response，按状态码判定。
		if retryableStatus(resp.StatusCode) {
			// 429 优先读 Retry-After 头；5xx 用指数退避。
			wait := backoff(attempt)
			if resp.StatusCode == 429 {
				if ra := parseRetryAfter(resp); ra > 0 {
					wait = ra
				}
			}
			// 关掉这次的 body 再决定是否重试（避免连接泄漏）。
			resp.Body.Close()

			logger.Info("可重试状态码",
				"attempt", attempt+1, "status", resp.StatusCode, "wait", wait)
			if attempt >= maxRetries {
				return nil, fmt.Errorf("响应状态 %d（重试 %d 次后仍失败）", resp.StatusCode, attempt+1)
			}
			if !sleep(ctx, wait) {
				return nil, ctx.Err()
			}
			continue
		}

		// 情况 3：2xx/3xx/不可重试的 4xx（401/400/404）→ 直接返回，调用方判业务成功。
		return resp, nil
	}
}

// retryableError 判断网络错误是否值得重试。
//
// 重试：连接拒绝、连接重置、DNS 解析临时失败、超时（context.DeadlineExceeded）。
// 不重试：用户主动取消（context.Canceled）—— 取消就该立即停，不该重试。
func retryableError(err error) bool {
	if err == nil {
		return false
	}
	// 用户取消不重试（主动行为）。
	if errors.Is(err, context.Canceled) {
		return false
	}
	// 超时可重试（可能是网络慢，下次就好）。
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	// url.Error：包装了底层网络错误，按其临时性判定。
	var ue *url.Error
	if errors.As(err, &ue) {
		// 连接拒绝 / 重置等 syscall 错误可重试。
		if errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) ||
			errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ETIMEDOUT) {
			return true
		}
		// url.Error.Timeout() = 超时类，可重试。
		if ue.Timeout() {
			return true
		}
		// 临时错误（net.OpError.Temporary）可重试。
		// 注：Go 1.18+ 的 Temporary() 已废弃但 net 包仍返回，保守判定。
		return ue.Temporary()
	}
	return false
}

// backoff 计算第 attempt 次重试的退避时长（指数 + jitter）。
// attempt=0→1s, 1→2s, 2→4s，各 ±20% 抖动。
func backoff(attempt int) time.Duration {
	d := baseBackoff << attempt // base × 2^attempt
	if d <= 0 {
		d = baseBackoff // 防溢出（attempt 极大时）
	}
	// ±jitterFactor 抖动：防多客户端在限流恢复瞬间同步重试。
	jitter := time.Duration(float64(d) * jitterFactor * (rand.Float64()*2 - 1))
	return d + jitter
}

// parseRetryAfter 解析 Retry-After 响应头。
// 支持两种格式：
//   - 秒数："120"（HTTP/1.1 常见）
//   - HTTP-date："Wed, 21 Oct 2026 07:28:00 GMT"（RFC 7231）
//
// 解析失败或头不存在返回 -1（调用方退回指数退避）。
func parseRetryAfter(resp *http.Response) time.Duration {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return -1
	}
	// 尝试秒数。
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	// 尝试 HTTP-date。
	if t, err := http.ParseTime(v); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return -1
}

// sleep 在退避期间监听 ctx.Done()，被取消时立即返回 false。
// 正常睡满返回 true。
func sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
