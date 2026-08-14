package security

import "strings"

const backgroundJobStdioRedirect = " </dev/null >/dev/null 2>&1"

// findStandaloneAmpersandPositions 返回不在引号内的独立 & 下标（排除 &&）。
func findStandaloneAmpersandPositions(command string) []int {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}

	var positions []int
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command); i++ {
		r := command[i]
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			continue
		}
		if r == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if r != '&' || inSingleQuote || inDoubleQuote {
			continue
		}
		if i+1 < len(command) && command[i+1] == '&' {
			continue
		}
		if i > 0 && command[i-1] == '&' {
			continue
		}

		isStandalone := i == 0
		if !isStandalone {
			prev := command[i-1]
			isStandalone = prev == ' ' || prev == '\t' || prev == '\n' || prev == '\r'
		}
		if !isStandalone {
			continue
		}
		if i == len(command)-1 {
			positions = append(positions, i)
			continue
		}
		next := command[i+1]
		if next == ' ' || next == '\t' || next == '\n' || next == '\r' {
			positions = append(positions, i)
		}
	}
	return positions
}

func segmentHasStdioRedirect(segment string) bool {
	lower := strings.ToLower(strings.TrimSpace(segment))
	if lower == "" {
		return false
	}
	if strings.Contains(lower, ">/dev/null") || strings.Contains(lower, "2>/dev/null") {
		return true
	}
	if strings.Contains(lower, "&>") || strings.Contains(lower, "&>>") {
		return true
	}
	if strings.Contains(lower, "2>&1") && strings.Contains(lower, "/dev/null") {
		return true
	}
	return false
}

// RedirectBackgroundJobStdio 为每个独立 & 前的后台段注入 </dev/null >/dev/null 2>&1，
// 避免后台子进程占用 execute/exec 管道导致挂死。
func RedirectBackgroundJobStdio(command string) string {
	positions := findStandaloneAmpersandPositions(command)
	if len(positions) == 0 {
		return command
	}

	out := command
	for j := len(positions) - 1; j >= 0; j-- {
		i := positions[j]
		before := out[:i]
		after := out[i:]
		trimmed := strings.TrimRight(before, " \t\r\n")
		if segmentHasStdioRedirect(trimmed) {
			continue
		}
		trailing := before[len(trimmed):]
		out = trimmed + backgroundJobStdioRedirect + trailing + after
	}
	return out
}

// PrepareShellCommandForExecute 组合 execute/exec 用的非交互包装与后台 IO 重定向。
// 须先注入 exec </dev/null，再改写 & 后台段，否则段内 </dev/null 会使 stdin 重定向被误判为已存在。
func PrepareShellCommandForExecute(shellCommand string) string {
	return RedirectBackgroundJobStdio(PrepareNonInteractiveShellCommand(shellCommand))
}
