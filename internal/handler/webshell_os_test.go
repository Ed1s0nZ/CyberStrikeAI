package handler

import (
	"encoding/base64"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func newTestWebShellHandler() *WebShellHandler {
	return NewWebShellHandler(zap.NewNop(), nil)
}

func TestNormalizeWebshellOS(t *testing.T) {
	cases := map[string]string{
		"":        "auto",
		"  ":      "auto",
		"auto":    "auto",
		"AUTO":    "auto",
		"linux":   "linux",
		"Linux":   "linux",
		"windows": "windows",
		"WINDOWS": "windows",
		"macos":   "auto", // 未支持的回退 auto
		"solaris": "auto",
	}
	for in, want := range cases {
		if got := normalizeWebshellOS(in); got != want {
			t.Errorf("normalizeWebshellOS(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveWebshellOS(t *testing.T) {
	type testCase struct {
		osTag     string
		shellType string
		want      string
	}
	cases := []testCase{
		// 显式 OS：按用户选择，忽略 shellType
		{"linux", "asp", "linux"},
		{"windows", "php", "windows"},
		{"LINUX", "jsp", "linux"},

		// auto + 各种 shellType：asp/aspx → windows，其他 → linux
		{"auto", "asp", "windows"},
		{"auto", "aspx", "windows"},
		{"auto", "ASP", "windows"},
		{"auto", "php", "linux"},
		{"auto", "jsp", "linux"},
		{"auto", "custom", "linux"},
		{"auto", "", "linux"},

		// 空/未知 OS 等价 auto
		{"", "asp", "windows"},
		{"", "php", "linux"},
		{"unknown", "aspx", "windows"},
	}
	for _, c := range cases {
		got := resolveWebshellOS(c.osTag, c.shellType)
		if got != c.want {
			t.Errorf("resolveWebshellOS(%q,%q) = %q, want %q", c.osTag, c.shellType, got, c.want)
		}
	}
}

func TestQuoteCmdPath(t *testing.T) {
	cases := map[string]string{
		"":                     `"."`,
		`C:\Windows\Temp`:      `"C:\Windows\Temp"`,
		`C:\Program Files\a`:   `"C:\Program Files\a"`,
		`C:\weird"name\f.txt`:  `"C:\weird""name\f.txt"`,
		`.`:                    `"."`,
	}
	for in, want := range cases {
		if got := quoteCmdPath(in); got != want {
			t.Errorf("quoteCmdPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuoteShellSinglePosix(t *testing.T) {
	cases := map[string]string{
		"":             ".",
		"/tmp/a b":     "'/tmp/a b'",
		"/tmp/it's.txt": `'/tmp/it'\''s.txt'`,
	}
	for in, want := range cases {
		if got := quoteShellSinglePosix(in); got != want {
			t.Errorf("quoteShellSinglePosix(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestBuildFileCommand_LinuxBranch 覆盖 Linux 目标下每个 action 产出的命令
func TestBuildFileCommand_LinuxBranch(t *testing.T) {
	h := newTestWebShellHandler()
	base := fileCommandInput{OS: "linux", ShellType: "php"}

	mustContain := func(t *testing.T, cmd string, substrings ...string) {
		t.Helper()
		for _, s := range substrings {
			if !strings.Contains(cmd, s) {
				t.Errorf("expected command to contain %q, got: %s", s, cmd)
			}
		}
	}
	mustNotContain := func(t *testing.T, cmd string, substrings ...string) {
		t.Helper()
		for _, s := range substrings {
			if strings.Contains(cmd, s) {
				t.Errorf("command should not contain %q, got: %s", s, cmd)
			}
		}
	}

	// list with empty path defaults to '.'
	in := base
	in.Action = "list"
	cmd, err := h.buildFileCommand(in)
	if err != nil {
		t.Fatalf("list linux: unexpected err: %v", err)
	}
	mustContain(t, cmd, "ls -la", "'.'")

	// list with path containing spaces
	in.Path = "/tmp/my files"
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "ls -la ", "'/tmp/my files'")

	// read with path
	in = base
	in.Action = "read"
	in.Path = "/etc/passwd"
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "cat ", "'/etc/passwd'")

	// read without path → error
	in.Path = ""
	if _, err := h.buildFileCommand(in); err != errFileOpPathRequired {
		t.Errorf("read empty path: want errFileOpPathRequired, got %v", err)
	}

	// delete
	in = base
	in.Action = "delete"
	in.Path = "/tmp/a.txt"
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "rm -f ", "'/tmp/a.txt'")
	mustNotContain(t, cmd, "del")

	// mkdir
	in.Action = "mkdir"
	in.Path = "/tmp/new/sub"
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "mkdir -p ", "'/tmp/new/sub'")

	// rename
	in = base
	in.Action = "rename"
	in.Path = "/tmp/a"
	in.TargetPath = "/tmp/b"
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "mv -f ", "'/tmp/a'", "'/tmp/b'")

	// rename missing target → error
	in.TargetPath = ""
	if _, err := h.buildFileCommand(in); err != errFileOpRenameNeedsBothPaths {
		t.Errorf("rename empty target: want errFileOpRenameNeedsBothPaths, got %v", err)
	}

	// write
	in = base
	in.Action = "write"
	in.Path = "/tmp/w.txt"
	in.Content = "hello 世界"
	cmd, _ = h.buildFileCommand(in)
	b64 := base64.StdEncoding.EncodeToString([]byte("hello 世界"))
	mustContain(t, cmd, "echo '"+b64+"'", "| base64 -d", "> '/tmp/w.txt'")

	// upload
	in = base
	in.Action = "upload"
	in.Path = "/tmp/bin"
	in.Content = "YWJjZA==" // base64 of "abcd"
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "echo 'YWJjZA=='", "| base64 -d", "> '/tmp/bin'")

	// upload oversized content → error
	in.Content = strings.Repeat("A", 513*1024)
	if _, err := h.buildFileCommand(in); err != errFileOpUploadTooLarge {
		t.Errorf("upload too large: want errFileOpUploadTooLarge, got %v", err)
	}

	// upload_chunk with chunk_index=0 uses single redirect
	in = base
	in.Action = "upload_chunk"
	in.Path = "/tmp/bin"
	in.Content = "YWJj"
	in.ChunkIndex = 0
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "base64 -d > '/tmp/bin'")
	mustNotContain(t, cmd, ">>")

	// upload_chunk with chunk_index>0 uses append redirect
	in.ChunkIndex = 1
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "base64 -d >> '/tmp/bin'")

	// unsupported action
	in = base
	in.Action = "nope"
	if _, err := h.buildFileCommand(in); err == nil || !strings.Contains(err.Error(), "unsupported action") {
		t.Errorf("unknown action: want unsupported action error, got %v", err)
	}
}

// TestBuildFileCommand_WindowsBranch 覆盖 Windows 目标下每个 action 产出的命令
func TestBuildFileCommand_WindowsBranch(t *testing.T) {
	h := newTestWebShellHandler()
	base := fileCommandInput{OS: "windows", ShellType: "php"}

	mustContain := func(t *testing.T, cmd string, substrings ...string) {
		t.Helper()
		for _, s := range substrings {
			if !strings.Contains(cmd, s) {
				t.Errorf("expected command to contain %q, got: %s", s, cmd)
			}
		}
	}
	mustNotContain := func(t *testing.T, cmd string, substrings ...string) {
		t.Helper()
		for _, s := range substrings {
			if strings.Contains(cmd, s) {
				t.Errorf("command should not contain %q, got: %s", s, cmd)
			}
		}
	}

	// list
	in := base
	in.Action = "list"
	cmd, _ := h.buildFileCommand(in)
	mustContain(t, cmd, "dir /a ", `"."`)
	mustNotContain(t, cmd, "ls -la")

	in.Path = `C:\Users\Public Docs`
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "dir /a ", `"C:\Users\Public Docs"`)

	// read
	in = base
	in.Action = "read"
	in.Path = `C:\flag.txt`
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "type ", `"C:\flag.txt"`)

	// delete
	in.Action = "delete"
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "del /q /f ", `"C:\flag.txt"`)
	mustNotContain(t, cmd, "rm -f")

	// mkdir
	in.Action = "mkdir"
	in.Path = `C:\a\b\c`
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "md ", `"C:\a\b\c"`)

	// rename
	in = base
	in.Action = "rename"
	in.Path = `C:\a.txt`
	in.TargetPath = `C:\b.txt`
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "move /y ", `"C:\a.txt"`, `"C:\b.txt"`)

	// write → PowerShell base64 one-liner
	in = base
	in.Action = "write"
	in.Path = `C:\out.txt`
	in.Content = "hello 世界"
	cmd, _ = h.buildFileCommand(in)
	wantB64 := base64.StdEncoding.EncodeToString([]byte("hello 世界"))
	mustContain(t, cmd,
		"powershell -NoProfile -NonInteractive -Command",
		"[Convert]::FromBase64String('"+wantB64+"')",
		"[IO.File]::WriteAllBytes('C:\\out.txt'",
	)
	mustNotContain(t, cmd, "echo ", "base64 -d")

	// upload (chunk_index=0 equivalent) uses WriteAllBytes
	in = base
	in.Action = "upload"
	in.Path = `C:\bin\f`
	in.Content = "YWJjZA=="
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "WriteAllBytes('C:\\bin\\f'", "FromBase64String('YWJjZA==')")

	// upload_chunk index=0 → WriteAllBytes
	in.Action = "upload_chunk"
	in.ChunkIndex = 0
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "WriteAllBytes(")
	mustNotContain(t, cmd, "FileMode]::Append")

	// upload_chunk index>0 → append (Open with Append mode)
	in.ChunkIndex = 1
	cmd, _ = h.buildFileCommand(in)
	mustContain(t, cmd, "[IO.FileMode]::Append", "FromBase64String('YWJjZA==')")
}

// TestBuildFileCommand_AutoFallbackMatchesLegacyBehavior 确保 os=auto 时与旧版 shellType 判定行为完全一致
// asp/aspx 视为 Windows（旧行为），其他视为 Linux。
func TestBuildFileCommand_AutoFallbackMatchesLegacyBehavior(t *testing.T) {
	h := newTestWebShellHandler()

	// asp + auto → windows 命令
	cmd, _ := h.buildFileCommand(fileCommandInput{Action: "list", OS: "auto", ShellType: "asp"})
	if !strings.Contains(cmd, "dir /a") {
		t.Errorf("auto + asp should use Windows cmd, got: %s", cmd)
	}

	cmd, _ = h.buildFileCommand(fileCommandInput{Action: "list", OS: "auto", ShellType: "aspx"})
	if !strings.Contains(cmd, "dir /a") {
		t.Errorf("auto + aspx should use Windows cmd, got: %s", cmd)
	}

	// php/jsp/custom + auto → linux 命令（与历史行为一致）
	for _, st := range []string{"php", "jsp", "custom", ""} {
		cmd, _ = h.buildFileCommand(fileCommandInput{Action: "list", OS: "auto", ShellType: st})
		if !strings.Contains(cmd, "ls -la") {
			t.Errorf("auto + %q should use Linux cmd, got: %s", st, cmd)
		}
	}

	// 显式 OS 覆盖 shellType
	cmd, _ = h.buildFileCommand(fileCommandInput{Action: "list", OS: "windows", ShellType: "php"})
	if !strings.Contains(cmd, "dir /a") {
		t.Errorf("explicit windows should override php shellType, got: %s", cmd)
	}
	cmd, _ = h.buildFileCommand(fileCommandInput{Action: "list", OS: "linux", ShellType: "asp"})
	if !strings.Contains(cmd, "ls -la") {
		t.Errorf("explicit linux should override asp shellType, got: %s", cmd)
	}
}
