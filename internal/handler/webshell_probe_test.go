package handler

import "testing"

func TestClassifyWebshellOSProbeOutput(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"Windows cmd 回显完整", ":OSPROBE_Windows_NT:END\r\n", "windows"},
		{"Windows cmd 回显带额外空行", "\r\n:OSPROBE_Windows_NT:END\r\n", "windows"},
		{"Windows 次级线索 - ver banner", "Microsoft Windows [版本 10.0.19045]\r\n", "windows"},
		{"Linux sh 字面量回显", ":OSPROBE_%OS%:END\n", "linux"},
		{"Linux 紧凑输出（无换行）", ":OSPROBE_%OS%:END", "linux"},
		{"空输出 - 无法判定", "", ""},
		{"被过滤的输出 - 无法判定", "something weird", ""},
		{"仅有 OSPROBE 前缀但被截断 - 保守返回空", ":OSPROBE_:END", ""},
	}
	for _, c := range cases {
		if got := classifyWebshellOSProbeOutput(c.in); got != c.want {
			t.Errorf("case %q: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestProbeWebshellOSViaExec_SendsOneCommandOnly(t *testing.T) {
	var calls []string
	fn := func(cmd string) (string, bool) {
		calls = append(calls, cmd)
		return ":OSPROBE_Windows_NT:END", true
	}
	got := probeWebshellOSViaExec(fn)
	if got != "windows" {
		t.Fatalf("want windows, got %q", got)
	}
	if len(calls) != 1 {
		t.Fatalf("probe should issue exactly one exec call, got %d: %v", len(calls), calls)
	}
	if calls[0] != webshellOSProbeCommand {
		t.Errorf("probe command mismatch: got %q", calls[0])
	}
}

func TestProbeWebshellOSViaExec_NotOkReturnsEmpty(t *testing.T) {
	// HTTP 非 200 的场景：execFn 返回 ok=false，探活应放弃
	fn := func(cmd string) (string, bool) { return "whatever", false }
	if got := probeWebshellOSViaExec(fn); got != "" {
		t.Errorf("want empty when exec not ok, got %q", got)
	}
}

func TestProbeWebshellOSViaExec_NilSafeguard(t *testing.T) {
	if got := probeWebshellOSViaExec(nil); got != "" {
		t.Errorf("nil execFn should return empty, got %q", got)
	}
}

func TestProbeWebshellOSViaExec_LinuxUname(t *testing.T) {
	// 某些 webshell 对 `%OS%` 字面量也会过滤（例如安全规则），
	// 但主要路径是"%OS% 字面量被原样回显"。这里覆盖标准 Linux 场景。
	fn := func(cmd string) (string, bool) {
		return ":OSPROBE_%OS%:END\n", true
	}
	if got := probeWebshellOSViaExec(fn); got != "linux" {
		t.Errorf("Linux case: want linux, got %q", got)
	}
}
