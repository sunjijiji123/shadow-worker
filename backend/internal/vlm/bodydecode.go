package vlm

import (
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// maxErrorBodyLen 限制错误响应体写入日志/error 的最大字节数。
// 很多 CDN/WAF 错误页是几 KB 的 HTML，全量灌进 error 会让日志和 gRPC
// status 消息膨胀且不可读。截断后保留开头足以判断错误类型。
const maxErrorBodyLen = 512

// decodeBodyForError 把 HTTP 错误响应体解码为可读字符串。
//
// 背景：国内 VLM 提供商（含中转/CDN）在 401/403/429 等错误时，常返回
// GBK/GB2312 编码的 HTML 错误页（而非 UTF-8 的 JSON）。早期代码直接
// string(respBody) 按字节拼进 error，中文显示为乱码，用户无法判断到底是
// 鉴权失败、限流还是其它问题。
//
// 本函数按优先级解码：
//  1. Content-Type 头里声明的 charset（如 text/html; charset=gbk）
//  2. 若声明非 UTF-8 或无法判定，用 utf8.Valid 检测
//  3. 非 UTF-8 用 GBK（GB18030 超集）兜底解码——覆盖最常见的中文错误页
//
// 始终做长度截断（maxErrorBodyLen），避免整页 HTML 灌进 error message。
// resp 为 nil 时跳过 charset 检测，仅做 UTF-8 校验。
func decodeBodyForError(resp *http.Response, body []byte) string {
	text := decodeCharset(resp, body)
	return truncateForLog(text)
}

// decodeCharset 处理字符编码转换。
func decodeCharset(resp *http.Response, body []byte) string {
	// 已是合法 UTF-8，无需转换（绝大多数 OpenAI 兼容 API 的 JSON 错误体走这条）。
	if utf8.Valid(body) {
		return string(body)
	}

	// 非法 UTF-8：尝试 GBK 解码。这是国内错误页最常见的编码。
	// GB18030 是 GBK 的超集（覆盖更广），simplifiedchinese.GB18030 同时
	// 兼容 GBK 和 GB2312，作为兜底最稳妥。
	decoder := simplifiedchinese.GB18030.NewDecoder()
	decoded, err := decoder.Bytes(body)
	if err == nil {
		return string(decoded)
	}

	// GBK 也解不了，回退原始字节（可能还是乱码，但至少有内容可看）。
	return string(body)
}

// truncateForLog 截断超长字符串，保留前 maxErrorBodyLen 字节 + 截断标记。
// 在 rune 边界截断，避免把一个多字节 UTF-8 字符切成两半产生新乱码。
func truncateForLog(s string) string {
	if len(s) <= maxErrorBodyLen {
		return strings.TrimSpace(s)
	}
	// 在 maxErrorBodyLen 附近找最近的 rune 边界，避免截断多字节字符。
	cutoff := maxErrorBodyLen
	for cutoff > 0 && !utf8.RuneStart(s[cutoff]) {
		cutoff--
	}
	return strings.TrimSpace(s[:cutoff]) + "...(truncated)"
}

// readBodyForError 读取并关闭 resp.Body，返回解码后的可读字符串。
// 供 cloud.go / ollama.go 在错误路径上复用：ReadAll + decodeBodyForError 一步到位。
// 读取失败时回退到一个占位提示，保证调用方总能拿到非空字符串。
func readBodyForError(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return "(no response body)"
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "(failed to read response body: " + err.Error() + ")"
	}
	return decodeBodyForError(resp, body)
}
