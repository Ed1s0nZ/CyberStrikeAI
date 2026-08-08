package security

// 集成测试：走 ExecuteTool("exec") → executeSystemCommand 真实执行路径，
// 验证危险命令拦截在默认开启 / 自定义规则 / 整体关闭 三种配置下的行为，
// 并确认安全命令不受影响。

import (
	"context"
	"strings"
	"testing"

	"cyberstrike-ai/internal/config"

	"go.uber.org/zap"
)

func boolPtr(b bool) *bool { return &b }

func execToolResult(t *testing.T, cfg *config.SecurityConfig, command string) (string, bool) {
	t.Helper()
	exe := NewExecutor(cfg, nil, zap.NewNop())
	res, err := exe.ExecuteTool(context.Background(), "exec", map[string]interface{}{"command": command})
	if err != nil {
		t.Fatalf("ExecuteTool err=%v", err)
	}
	text := ""
	for _, c := range res.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}
	return text, res.IsError
}

func TestExecDangerousCommandInterceptDefaultEnabled(t *testing.T) {
	// 默认配置（nil DangerousCommandEnabled 视为开启）：破坏性命令被拦截
	for _, cmd := range []string{
		"rm -rf /tmp/x",
		"del /f /s /q C:\\temp",
		"format C:",
		"shutdown /s /t 0",
		"Stop-Service spooler",
	} {
		text, isErr := execToolResult(t, &config.SecurityConfig{}, cmd)
		if !isErr || !strings.Contains(text, "已拦截危险命令") {
			t.Errorf("cmd=%q 应被拦截, isErr=%v text=%q", cmd, isErr, text)
		}
	}
}

func TestExecDangerousCommandCustomBlocklist(t *testing.T) {
	cfg := &config.SecurityConfig{
		DangerousCommandEnabled:    boolPtr(true),
		DangerousCommandBlocklist: []string{`\bnuke\b`},
	}
	text, isErr := execToolResult(t, cfg, "my-weird-tool --nuke")
	if !isErr || !strings.Contains(text, "自定义规则") {
		t.Errorf("自定义规则应生效, isErr=%v text=%q", isErr, text)
	}
	// 未命中自定义规则的安全命令应正常执行
	text, isErr = execToolResult(t, cfg, "echo CSATEST-OK")
	if isErr || !strings.Contains(text, "CSATEST-OK") {
		t.Errorf("安全命令不应受影响, isErr=%v text=%q", isErr, text)
	}
}

func TestExecSafeCommandStillWorks(t *testing.T) {
	text, isErr := execToolResult(t, &config.SecurityConfig{DangerousCommandEnabled: boolPtr(true)}, "echo CSATEST-OK")
	if isErr || !strings.Contains(text, "CSATEST-OK") {
		t.Errorf("安全命令应正常执行, isErr=%v text=%q", isErr, text)
	}
}

func TestExecDangerousCommandDisabled(t *testing.T) {
	// dangerous_command_enabled=false 时整体关闭（不推荐）：破坏性命令不再被拦截。
	// 使用无副作用命令 Stop-Service 不存在的服务验证放行路径，避免真实执行破坏性命令。
	cfg := &config.SecurityConfig{DangerousCommandEnabled: boolPtr(false)}
	text, isErr := execToolResult(t, cfg, "Stop-Service NonexistentSvc123")
	if strings.Contains(text, "已拦截危险命令") {
		t.Errorf("关闭模式下不应拦截: text=%q", text)
	}
	if !isErr {
		t.Errorf("命令应执行并因服务不存在而报错（证明未被拦截而是真实执行了）: text=%q", text)
	}
}
