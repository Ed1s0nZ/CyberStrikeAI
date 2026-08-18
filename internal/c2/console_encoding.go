package c2

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// NormalizeConsoleOutput 将 implant/Shell 原始控制台字节转为 UTF-8 文本。
// osTag 来自会话的 os 字段（如 windows / Windows 10）；空值时按 auto 处理。
func NormalizeConsoleOutput(raw []byte, osTag string) string {
	if len(raw) == 0 {
		return ""
	}
	osTag = strings.ToLower(strings.TrimSpace(osTag))
	isWindows := strings.Contains(osTag, "windows")

	if utf8.Valid(raw) {
		return string(raw)
	}
	if isWindows {
		if out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), raw); err == nil {
			return string(out)
		}
	}
	// 非 Windows 或解码失败：GB18030 兜底（覆盖 GBK）
	if out, _, err := transform.Bytes(simplifiedchinese.GB18030.NewDecoder(), raw); err == nil {
		return string(out)
	}
	return string(raw)
}

// ResolveTaskResultText 合并 beacon 回传的 Output/OutputB64（及 Error/ErrorB64），按会话 OS 解码。
func ResolveTaskResultText(plain, b64, sessionOS string) string {
	if strings.TrimSpace(b64) != "" {
		raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
		if err == nil {
			return NormalizeConsoleOutput(raw, sessionOS)
		}
	}
	if plain == "" {
		return ""
	}
	return NormalizeConsoleOutput([]byte(plain), sessionOS)
}
