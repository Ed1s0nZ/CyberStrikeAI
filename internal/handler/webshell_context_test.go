package handler

import (
	"strings"
	"testing"

	"cyberstrike-ai/internal/database"
)

func TestBuildWebshellAssistantContext_WindowsExplicit(t *testing.T) {
	conn := &database.WebShellConnection{
		ID:       "ws_win01",
		Remark:   "IIS Windows 靶机",
		URL:      "http://example.com/shell.php",
		Type:     "php",
		OS:       "windows",
		Encoding: "gbk",
	}
	got := BuildWebshellAssistantContext(conn, WebshellSkillHintDefault, "列出当前目录并告诉我 flag 在哪")

	mustContain(t, got,
		"[WebShell 助手上下文]",
		"ws_win01",
		"IIS Windows 靶机",
		"目标系统：Windows",
		"dir /a",
		"move /y",
		"避免 ls / cat / rm",
		"响应编码：GBK",
		"后端已自动转码为 UTF-8",
		"connection_id 填 \"ws_win01\"",
		"webshell_exec、webshell_file_list",
		WebshellSkillHintDefault,
		"用户请求：列出当前目录并告诉我 flag 在哪",
	)
	// Windows 场景下不应出现 Linux 命令推荐
	mustNotContain(t, got, "推荐 sh/bash")
}

func TestBuildWebshellAssistantContext_LinuxAutoFromPHP(t *testing.T) {
	conn := &database.WebShellConnection{
		ID:       "ws_lnx01",
		Remark:   "", // 测试备注为空时 fallback URL
		URL:      "http://example.com/a.php",
		Type:     "php",
		OS:       "auto", // auto + php → linux
		Encoding: "",     // auto 编码不显式提示
	}
	got := BuildWebshellAssistantContext(conn, WebshellSkillHintDefault, "看看 /etc/passwd")

	mustContain(t, got,
		"连接 ID：ws_lnx01",
		"备注：http://example.com/a.php", // 备注空时 fallback URL
		"目标系统：Linux/Unix",
		"ls -la",
		"mkdir -p",
		"避免 dir、type、del、move",
		"用户请求：看看 /etc/passwd",
	)
	// encoding=auto 不应出现"响应编码："这一行
	mustNotContain(t, got, "响应编码：")
	// Linux 场景不应出现 Windows 命令
	mustNotContain(t, got, "推荐 cmd/PowerShell")
}

func TestBuildWebshellAssistantContext_AutoFromASPDefaultsToWindows(t *testing.T) {
	// 保留向后兼容：旧连接没配 os，shellType=asp 时应视为 Windows
	conn := &database.WebShellConnection{
		ID:       "ws_asp01",
		Remark:   "老 ASP 靶机",
		Type:     "asp",
		OS:       "", // 空串等同 auto
		Encoding: "gb18030",
	}
	got := BuildWebshellAssistantContext(conn, WebshellSkillHintMultiAgent, "查当前用户")

	mustContain(t, got,
		"目标系统：Windows",
		"响应编码：GB18030",
		"后端已自动转码为 UTF-8 返回",
		WebshellSkillHintMultiAgent,
	)
	// 多代理 skill 文案里没有 DeepAgent，不应混入 default 文案
	mustNotContain(t, got, "DeepAgent")
}

func TestBuildWebshellAssistantContext_MultiAgentSkillHint(t *testing.T) {
	conn := &database.WebShellConnection{ID: "ws_m1", Remark: "x", Type: "php", OS: "linux"}
	got := BuildWebshellAssistantContext(conn, WebshellSkillHintMultiAgent, "hi")
	mustContain(t, got, WebshellSkillHintMultiAgent)
	mustNotContain(t, got, "DeepAgent")
}

func TestBuildWebshellAssistantContext_DefaultSkillHintFallback(t *testing.T) {
	conn := &database.WebShellConnection{ID: "ws_d1", Remark: "x", Type: "php", OS: "linux"}
	// skillHint 传空字符串时应回退到 default
	got := BuildWebshellAssistantContext(conn, "", "hi")
	mustContain(t, got, WebshellSkillHintDefault)
}

func TestBuildWebshellAssistantContext_UTF8EncodingIsAnnotated(t *testing.T) {
	conn := &database.WebShellConnection{
		ID: "ws_u1", Remark: "u", Type: "jsp", OS: "linux", Encoding: "utf-8",
	}
	got := BuildWebshellAssistantContext(conn, WebshellSkillHintDefault, "hi")
	mustContain(t, got, "响应编码：UTF-8", "目标原生 UTF-8")
}

func TestBuildWebshellAssistantContext_NilConnReturnsUserMsg(t *testing.T) {
	// 防御性：conn == nil 时不 panic，直接返回原消息
	got := BuildWebshellAssistantContext(nil, WebshellSkillHintDefault, "just the message")
	if got != "just the message" {
		t.Errorf("nil conn should return userMsg as-is, got %q", got)
	}
}

func TestDescribeTargetOSForPrompt(t *testing.T) {
	cases := map[string][]string{
		"windows": {"Windows", "dir /a", "move /y", "PowerShell"},
		"linux":   {"Linux/Unix", "ls -la", "mkdir -p"},
		"":        {"未知", "uname"}, // 防御性分支
	}
	for in, wants := range cases {
		got := describeTargetOSForPrompt(in)
		for _, w := range wants {
			if !strings.Contains(got, w) {
				t.Errorf("describeTargetOSForPrompt(%q) should contain %q, got: %s", in, w, got)
			}
		}
	}
}

func TestDescribeEncodingForPrompt(t *testing.T) {
	cases := map[string]string{
		"utf-8":   "UTF-8",
		"gbk":     "GBK",
		"gb18030": "GB18030",
		"auto":    "",
		"":        "",
	}
	for in, want := range cases {
		got := describeEncodingForPrompt(in)
		if want == "" && got != "" {
			t.Errorf("describeEncodingForPrompt(%q) should return empty string, got: %s", in, got)
		}
		if want != "" && !strings.Contains(got, want) {
			t.Errorf("describeEncodingForPrompt(%q) should contain %q, got: %s", in, want, got)
		}
	}
}

// ---- 小工具 ----

func mustContain(t *testing.T, text string, substrings ...string) {
	t.Helper()
	for _, s := range substrings {
		if !strings.Contains(text, s) {
			t.Errorf("expected text to contain %q\n--- text ---\n%s", s, text)
		}
	}
}

func mustNotContain(t *testing.T, text string, substrings ...string) {
	t.Helper()
	for _, s := range substrings {
		if strings.Contains(text, s) {
			t.Errorf("text should not contain %q\n--- text ---\n%s", s, text)
		}
	}
}
