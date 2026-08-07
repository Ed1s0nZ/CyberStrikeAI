package config

import (
	"strings"
	"testing"
)

func TestDefaultHitlAuditAgentPromptIncludesPrioritizedRules(t *testing.T) {
	prompt := DefaultHitlAuditAgentPrompt()
	for _, want := range []string{
		"只判断“当前最外层工具调用”",
		"事实来源只有 toolName 与 arguments/argumentsObj",
		"R1 身份管理变更",
		"R2 配置/服务变更",
		"R3 既有对象破坏",
		"R4 真实业务动作",
		"R5 可用性破坏",
		"上传新的 PHP/JSP/ASP WebShell",
		"SSH/HTTP 爆破：approve A3",
		"不得让低优先级 A 规则覆盖已命中的 R",
		"命中规则：R1|R2|R3|R4|R5|A1|A2|A3|A4|A5|A6|A7|D1",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("default approval prompt missing %q", want)
		}
	}
	if strings.Contains(prompt, `"editedArguments":{...}`) {
		t.Fatal("approval prompt must not request editedArguments")
	}
}

func TestDefaultHitlAuditAgentPromptReviewEditKeepsEditedArguments(t *testing.T) {
	prompt := DefaultHitlAuditAgentPromptReviewEdit()
	if !strings.Contains(prompt, `"editedArguments":{...}`) {
		t.Fatal("review-edit prompt must preserve editedArguments output")
	}
	if !strings.Contains(prompt, "命中规则：R1|R2|R3|R4|R5|A1|A2|A3|A4|A5|A6|A7|D1") {
		t.Fatal("review-edit prompt must require a matched rule")
	}
	if !strings.Contains(prompt, "只做最小必要修改以收窄范围、消除风险") {
		t.Fatal("review-edit prompt must preserve argument narrowing guidance")
	}
}
