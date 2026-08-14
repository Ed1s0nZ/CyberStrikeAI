package multiagent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/einomcp"
	"cyberstrike-ai/internal/security"

	localbk "github.com/cloudwego/eino-ext/adk/backend/local"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/filesystem"
	"github.com/cloudwego/eino/adk/middlewares/skill"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

func prepareEinoAgenticSkills(
	ctx context.Context,
	skillsDir string,
	ma *config.MultiAgentConfig,
	logger *zap.Logger,
) (loc *localbk.Local, skillMW adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage], fsTools bool, skillsRoot string, err error) {
	if ma == nil {
		return nil, nil, false, "", nil
	}
	needLocalBackend := ma.EinoMiddleware.ReductionEnable
	newLocalBackend := func() (*localbk.Local, error) {
		backend, backendErr := localbk.NewBackend(ctx, &localbk.Config{})
		if backendErr != nil {
			return nil, fmt.Errorf("eino local backend: %w", backendErr)
		}
		return backend, nil
	}
	if ma.EinoSkills.Disable {
		if !needLocalBackend {
			return nil, nil, false, "", nil
		}
		loc, err = newLocalBackend()
		return loc, nil, false, "", err
	}
	root := strings.TrimSpace(skillsDir)
	if root == "" {
		if logger != nil {
			logger.Warn("eino agentic skills: skills_dir empty, skip")
		}
		if !needLocalBackend {
			return nil, nil, false, "", nil
		}
		loc, err = newLocalBackend()
		return loc, nil, false, "", err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("skills_dir abs: %w", err)
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		if logger != nil {
			logger.Warn("eino agentic skills: directory missing, skip", zap.String("dir", abs), zap.Error(err))
		}
		if !needLocalBackend {
			return nil, nil, false, "", nil
		}
		loc, err = newLocalBackend()
		return loc, nil, false, "", err
	}

	loc, err = newLocalBackend()
	if err != nil {
		return nil, nil, false, "", err
	}

	skillBE, err := skill.NewBackendFromFilesystem(ctx, &skill.BackendFromFilesystemConfig{
		Backend: loc,
		BaseDir: abs,
	})
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("eino agentic skill filesystem backend: %w", err)
	}

	sc := &skill.TypedConfig[*schema.AgenticMessage]{Backend: skillBE}
	if name := strings.TrimSpace(ma.EinoSkills.SkillToolName); name != "" {
		sc.SkillToolName = &name
	}
	skillMW, err = skill.NewTyped[*schema.AgenticMessage](ctx, sc)
	if err != nil {
		return nil, nil, false, "", fmt.Errorf("eino agentic skill middleware: %w", err)
	}

	fsTools = ma.EinoSkills.EinoSkillFilesystemToolsEffective()
	return loc, skillMW, fsTools, abs, nil
}

func subAgentAgenticFilesystemMiddleware(
	ctx context.Context,
	loc *localbk.Local,
	invokeNotify *einomcp.ToolInvokeNotifyHolder,
	einoAgentName string,
	beginMonitor func(toolCallID, command string) string,
	appendPartialMonitor func(executionID, toolCallID, chunk string),
	registerCancelMonitor func(executionID string, cancel context.CancelFunc),
	unregisterCancelMonitor func(executionID string),
	finishMonitor func(executionID, toolCallID, command, stdout string, success bool, invokeErr error),
	toolTimeoutMinutes int,
	toolWaitTimeoutSeconds int,
	shellNoOutputTimeoutSec int,
	outputChunk func(toolName, toolCallID, chunk string),
) (adk.TypedChatModelAgentMiddleware[*schema.AgenticMessage], error) {
	if loc == nil {
		return nil, nil
	}
	return filesystem.NewTyped[*schema.AgenticMessage](ctx, &filesystem.MiddlewareConfig{
		Backend: loc,
		StreamingShell: &einoStreamingShellWrap{
			inner:                   security.NewEinoStreamingShell(),
			invokeNotify:            invokeNotify,
			einoAgentName:           strings.TrimSpace(einoAgentName),
			outputChunk:             outputChunk,
			beginMonitor:            beginMonitor,
			appendPartialMonitor:    appendPartialMonitor,
			registerCancelMonitor:   registerCancelMonitor,
			unregisterCancelMonitor: unregisterCancelMonitor,
			finishMonitor:           finishMonitor,
			toolTimeoutMinutes:      toolTimeoutMinutes,
			toolWaitTimeoutSeconds:  toolWaitTimeoutSeconds,
			shellNoOutputTimeoutSec: shellNoOutputTimeoutSec,
		},
	})
}

// agentToolTimeoutMinutes 返回 agent.tool_timeout_minutes（与 executeToolViaMCP 一致）；cfg 为 nil 时 0。
func agentToolTimeoutMinutes(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	return cfg.Agent.ToolTimeoutMinutes
}

func agentToolWaitTimeoutSeconds(cfg *config.Config) int {
	if cfg == nil {
		return 0
	}
	return cfg.Agent.ToolWaitTimeoutSeconds
}

// agentShellNoOutputTimeoutSeconds：0=默认 300s（5 分钟）；-1=关闭；>0=自定义秒数。
func agentShellNoOutputTimeoutSeconds(cfg *config.Config) int {
	if cfg == nil {
		return 300
	}
	v := cfg.Agent.ShellNoOutputTimeoutSeconds
	if v < 0 {
		return 0
	}
	if v == 0 {
		return 300
	}
	return v
}
