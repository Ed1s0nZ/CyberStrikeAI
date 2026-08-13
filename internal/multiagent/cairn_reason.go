package multiagent

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"go.uber.org/zap"

	"cyberstrike-ai/internal/config"
)

// cairnReasonGenInput 生成 Reason Agent 的模型输入：
// 注入 Fact-Intent-Hint 图 YAML + Reason 提示词（orchestrator-cairn.md）。
func cairnReasonGenInput(
	orchInstruction string,
	statePath string,
	appCfg *config.Config,
	logger *zap.Logger,
) adk.GenModelInput {
	oi := strings.TrimSpace(orchInstruction)
	return func(ctx context.Context, instruction string, _ *adk.AgentInput) ([]adk.Message, error) {
		// 加载当前状态图
		st, err := LoadCairnState(statePath)
		if err != nil {
			return nil, fmt.Errorf("cairn reason load state: %w", err)
		}
		graphYAML, err := st.ToYAML()
		if err != nil {
			return nil, fmt.Errorf("cairn reason marshal state: %w", err)
		}

		// 构建 Reason 提示词
		var sb strings.Builder
		if oi != "" {
			sb.WriteString(oi)
			sb.WriteString("\n\n")
		}
		sb.WriteString("## Graph\n```yaml\n")
		sb.WriteString(graphYAML)
		sb.WriteString("```\n\n")

		// 附加 valid_facts 和 open_intents（Cairn reason.md 的格式）
		sb.WriteString("## Valid facts\n```json\n")
		sb.WriteString(fmt.Sprintf("%v", st.FactIDs()))
		sb.WriteString("\n```\n\n")

		openIntents := st.OpenIntents()
		sb.WriteString("## Open Intents\n```json\n")
		if len(openIntents) == 0 {
			sb.WriteString("[]")
		} else {
			for i, it := range openIntents {
				if i > 0 {
					sb.WriteString(",\n")
				}
				sb.WriteString(fmt.Sprintf(`{"id": "%s", "from": %v, "description": "%s"}`,
					it.ID, it.From, strings.ReplaceAll(it.Description, `"`, `\"`)))
			}
		}
		sb.WriteString("\n```\n")

		msgs := []adk.Message{
			{Role: "system", Content: sb.String()},
		}
		if logger != nil {
			logger.Debug("cairn reason gen input",
				zap.String("state_path", statePath),
				zap.Int("facts", len(st.Facts)),
				zap.Int("open_intents", len(openIntents)),
			)
		}
		return msgs, nil
	}
}

// cairnExploreGenInput 生成 Explore Agent 的模型输入：
// 注入 Fact-Intent-Hint 图 YAML + 当前 intent + Explore 提示词（executor-cairn.md）。
func cairnExploreGenInput(
	execInstruction string,
	statePath string,
	intentID string,
	appCfg *config.Config,
	logger *zap.Logger,
) adk.GenModelInput {
	ei := strings.TrimSpace(execInstruction)
	return func(ctx context.Context, instruction string, _ *adk.AgentInput) ([]adk.Message, error) {
		// 加载当前状态图
		st, err := LoadCairnState(statePath)
		if err != nil {
			return nil, fmt.Errorf("cairn explore load state: %w", err)
		}
		graphYAML, err := st.ToYAML()
		if err != nil {
			return nil, fmt.Errorf("cairn explore marshal state: %w", err)
		}

		// 找当前 intent
		var currentIntent *Intent
		for i := range st.Intents {
			if st.Intents[i].ID == intentID {
				currentIntent = &st.Intents[i]
				break
			}
		}
		if currentIntent == nil {
			return nil, fmt.Errorf("cairn explore intent %s not found in state", intentID)
		}

		// 构建 Explore 提示词
		var sb strings.Builder
		if ei != "" {
			sb.WriteString(ei)
			sb.WriteString("\n\n")
		}
		sb.WriteString("## Graph\n```yaml\n")
		sb.WriteString(graphYAML)
		sb.WriteString("```\n\n")

		sb.WriteString("## Current Intent\n```\n")
		sb.WriteString(intentID)
		sb.WriteString("\n```\n\n")

		sb.WriteString("## Current Intent Description\n```\n")
		sb.WriteString(currentIntent.Description)
		sb.WriteString("\n```\n")

		msgs := []adk.Message{
			{Role: "system", Content: sb.String()},
		}
		if logger != nil {
			logger.Debug("cairn explore gen input",
				zap.String("state_path", statePath),
				zap.String("intent_id", intentID),
				zap.String("intent_desc", currentIntent.Description),
			)
		}
		return msgs, nil
	}
}
