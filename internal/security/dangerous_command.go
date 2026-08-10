package security

import (
	"regexp"
	"strings"
)

// 危险命令拦截（exec 工具全局兜底防线）。
//
// 背景：HITL 人机协同按会话开启，批量任务/机器人等路径不会强制审批；
// 一旦 AI 被提示词注入或目标输入误导，exec 可执行任意系统命令。
// 此处对破坏性命令做最后一道可配置的拦截，所有执行路径一律生效。
//
// 匹配方式：命令归一化（小写、压缩空白）后按正则匹配。
// 内置模式见 builtinDangerousPatterns；config.yaml 的
// security.dangerous_command_blocklist 可追加自定义正则，
// security.dangerous_command_enabled=false 可整体关闭（不推荐）。

var builtinDangerousPatterns = []struct {
	pattern *regexp.Regexp
	reason  string
}{
	// ---- Unix: 文件系统破坏 ----
	{regexp.MustCompile(`\brm\s+-(?:rf|fr|r\s+-f|f\s+-r)\b`), "rm -rf 递归强制删除"},
	{regexp.MustCompile(`\brm\s+-[a-z]*[rf][a-z]*\s+(?:/|~|\*|/\*|~/)\s*(?:$|[;&|])`), "rm 删除根目录/家目录/通配"},
	{regexp.MustCompile(`\bmkfs(?:\.[a-z0-9]+)?\b`), "mkfs 格式化文件系统"},
	{regexp.MustCompile(`\bdd\b[^\n;|&]*\bof=(?:/dev/|\\\\.\\)`), "dd 向块设备写入"},
	{regexp.MustCompile(`\bchmod\s+-r\s+[0-7]{3,4}\b`), "chmod -R 危险权限"},
	{regexp.MustCompile(`\bchown\s+-r\b`), "chown -R 递归变更属主"},
	{regexp.MustCompile(`\bumount\b`), "umount 卸载文件系统"},

	// ---- Unix: 系统关机/重启 ----
	{regexp.MustCompile(`\b(?:shutdown|reboot|halt|poweroff)\b`), "系统关机/重启"},
	{regexp.MustCompile(`\b(?:init|telinit)\s+0\b`), "init 0 停机"},

	// ---- Windows: 文件系统破坏 ----
	{regexp.MustCompile(`\bdel\b[^\n;|&]*\s/s\b`), "del /s 递归删除"},
	{regexp.MustCompile(`\brd\b[^\n;|&]*\s/s\b`), "rd /s 递归删除目录"},
	{regexp.MustCompile(`\brmdir\b[^\n;|&]*\s/s\b`), "rmdir /s 递归删除目录"},
	{regexp.MustCompile(`\bdel\b[^\n;|&]*\\windows\b`), "删除 Windows 系统目录"},
	{regexp.MustCompile(`\bremove-item\b[^\n;|&]*(?:-recurse|-r\b|-force|-f\b)`), "Remove-Item 递归/强制删除"},
	{regexp.MustCompile(`\bclear-content\b[^\n;|&]*(?:-path|-literalpath)`), "Clear-Content 清空文件"},
	{regexp.MustCompile(`\bformat\s+[a-z]:`), "format 格式化磁盘"},
	{regexp.MustCompile(`\bdiskpart\b`), "diskpart 磁盘分区"},
	{regexp.MustCompile(`\bchkdsk\b[^\n;|&]*\s/f\b`), "chkdsk /f 修复磁盘"},

	// ---- Windows: 进程/服务/注册表破坏 ----
	{regexp.MustCompile(`\btaskkill\b[^\n;|&]*\s/f\b`), "taskkill /f 强制终止进程"},
	{regexp.MustCompile(`\bstop-service\b`), "Stop-Service 停止服务"},
	{regexp.MustCompile(`\bstop-process\b`), "Stop-Process 终止进程"},
	{regexp.MustCompile(`\bsc\s+delete\b`), "sc delete 删除服务"},
	{regexp.MustCompile(`\breg\s+delete\b`), "reg delete 删除注册表项"},
	{regexp.MustCompile(`\bwmic\s+process\s+delete\b`), "wmic 删除进程"},
	{regexp.MustCompile(`\brmdir\b[^\n;|&]*\\windows\b`), "删除 Windows 系统目录"},

	// ---- 通用: 高危动作 ----
	{regexp.MustCompile(`\b(?:rm|del|rd|rmdir|remove-item)\b[^\n;|&]*(?:\b(?:boot|system32)\b|/etc|/boot|/var/lib|/usr)`), "删除系统关键目录"},
	{regexp.MustCompile(`\bgit\s+push\s+(?:-f|--force)\b`), "git 强制推送"},
	{regexp.MustCompile(`\bcipher\s+/w\b`), "cipher /w 擦除磁盘剩余空间"},
}

// normalizeCommand 归一化命令用于匹配：小写 + 压缩连续空白。
func normalizeCommand(cmd string) string {
	return strings.Join(strings.Fields(strings.ToLower(cmd)), " ")
}

// MatchDangerousCommand 检查命令是否命中危险黑名单。
// 命中时返回 (命中原因, true)；未命中返回 ("", false)。
// customPatterns 为 config 追加的自定义正则（小写不敏感），可为空。
func MatchDangerousCommand(cmd string, customPatterns []string) (string, bool) {
	norm := normalizeCommand(cmd)
	if norm == "" {
		return "", false
	}
	for _, d := range builtinDangerousPatterns {
		if d.pattern.MatchString(norm) {
			return d.reason, true
		}
	}
	for _, p := range customPatterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		re, err := regexp.Compile(p)
		if err != nil {
			continue // 非法正则忽略，避免配置错误导致全部拦截
		}
		if re.MatchString(norm) {
			return "自定义规则: " + p, true
		}
	}
	return "", false
}
