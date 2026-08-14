package multiagent

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/middlewares/summarization"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

const (
	toolOutputTruncationMarker = "\n\n...[tool output truncated; full text persisted in reduction cache or summarization transcript]...\n\n"
	aggressiveToolTruncDivisor = 4
)

// isEinoContextOverflowError reports API-side context window rejections.
func isEinoContextOverflowError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return false
	}
	markers := []string{
		"context length",
		"context_length",
		"maximum context",
		"max context",
		"context window",
		"context overflow",
		"too many tokens",
		"token limit",
		"tokens exceed",
		"exceeds the context",
		"input is too long",
		"prompt is too long",
		"request too large",
	}
	for _, m := range markers {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}

func truncateBytesWithMarker(content string, maxBytes int, marker string) string {
	if maxBytes <= 0 || len(content) <= maxBytes {
		return content
	}
	if marker == "" {
		marker = toolOutputTruncationMarker
	}
	budget := maxBytes - len(marker)
	if budget <= 0 {
		if len(marker) > maxBytes {
			return marker[:maxBytes]
		}
		return marker
	}
	head := budget / 2
	tail := budget - head
	for head > 0 && !utf8.RuneStart(content[head]) {
		head--
	}
	tailStart := len(content) - tail
	for tailStart < len(content) && !utf8.RuneStart(content[tailStart]) {
		tailStart++
	}
	return content[:head] + marker + content[tailStart:]
}

func cloneMessage(msg adk.Message) adk.Message {
	if msg == nil {
		return nil
	}
	cloned := *msg
	return &cloned
}

func truncateMessageToolContent(msg adk.Message, maxBytes int, spillRef string) adk.Message {
	if msg == nil || maxBytes <= 0 {
		return msg
	}
	out := cloneMessage(msg)
	marker := toolOutputTruncationMarker
	if spillRef != "" {
		marker = fmt.Sprintf("\n\n...[tool output truncated; retrieve full text via: %s]...\n\n", spillRef)
	}
	switch out.Role {
	case schema.Tool:
		out.Content = truncateBytesWithMarker(out.Content, maxBytes, marker)
	case schema.Assistant:
		if out.ReasoningContent != "" {
			out.ReasoningContent = truncateBytesWithMarker(out.ReasoningContent, maxBytes, marker)
		}
		if out.Content != "" {
			out.Content = truncateBytesWithMarker(out.Content, maxBytes, marker)
		}
	case schema.User:
		if out.Content != "" {
			out.Content = truncateBytesWithMarker(out.Content, maxBytes, marker)
		}
	}
	return out
}

func countMessagesTokens(
	ctx context.Context,
	msgs []adk.Message,
	counter summarization.TokenCounterFunc,
	tools []*schema.ToolInfo,
) (int, error) {
	if counter == nil {
		return 0, nil
	}
	n, err := counter(ctx, &summarization.TokenCounterInput{Messages: msgs, Tools: tools})
	if err != nil {
		return 0, err
	}
	return n, nil
}

func truncateRoundMessagesToTokenBudget(
	ctx context.Context,
	round messageRound,
	tokenBudget int,
	counter summarization.TokenCounterFunc,
	toolMaxBytes int,
	spillRef string,
) ([]adk.Message, error) {
	if tokenBudget <= 0 || len(round.messages) == 0 {
		return nil, nil
	}
	msgs := append([]adk.Message(nil), round.messages...)
	if n, err := countMessagesTokens(ctx, msgs, counter, nil); err != nil {
		return nil, err
	} else if n <= tokenBudget {
		return msgs, nil
	}
	if toolMaxBytes <= 0 {
		toolMaxBytes = 12000
	}
	for pass := 0; pass < 8 && toolMaxBytes >= 32; pass++ {
		out := make([]adk.Message, 0, len(msgs))
		for _, msg := range msgs {
			switch {
			case msg != nil && msg.Role == schema.Tool:
				out = append(out, truncateMessageToolContent(msg, toolMaxBytes, spillRef))
			case msg != nil && msg.Role == schema.Assistant:
				out = append(out, truncateMessageToolContent(msg, toolMaxBytes, spillRef))
			default:
				out = append(out, msg)
			}
		}
		n, err := countMessagesTokens(ctx, out, counter, nil)
		if err != nil {
			return nil, err
		}
		if n <= tokenBudget {
			return out, nil
		}
		msgs = out
		toolMaxBytes /= 2
	}
	return msgs, nil
}

type compactMessagesOpts struct {
	maxTokens    int
	counter      summarization.TokenCounterFunc
	toolMaxBytes int
	spillRef     string
	aggressive   bool
	logger       *zap.Logger
	phase        string
}

func compactMessagesByDroppingRounds(
	ctx context.Context,
	messages []adk.Message,
	opts compactMessagesOpts,
) ([]adk.Message, bool) {
	if opts.maxTokens <= 0 || len(messages) == 0 || opts.counter == nil {
		return messages, false
	}
	before, err := countMessagesTokens(ctx, messages, opts.counter, nil)
	if err != nil || before <= opts.maxTokens {
		return messages, false
	}

	systems := make([]adk.Message, 0, 1)
	contextMsgs := make([]adk.Message, 0, len(messages))
	for _, msg := range messages {
		if msg != nil && msg.Role == schema.System && len(contextMsgs) == 0 {
			systems = append(systems, msg)
			continue
		}
		if msg != nil {
			contextMsgs = append(contextMsgs, msg)
		}
	}
	rounds := splitMessagesIntoRounds(contextMsgs)
	if len(rounds) == 0 {
		return messages, false
	}

	startIdx := 0
	if opts.aggressive {
		startIdx = len(rounds) - 1
		if startIdx < 0 {
			startIdx = 0
		}
	}
	dropped := 0
	for len(rounds) > 1 || (opts.aggressive && len(rounds) == 1) {
		if !opts.aggressive && len(rounds) <= 1 {
			break
		}
		if opts.aggressive && len(rounds) == 1 {
			// Fall through to latest-round truncation below.
			break
		}
		rounds = rounds[1:]
		dropped++
		candidate := append([]adk.Message(nil), systems...)
		for _, round := range rounds {
			candidate = append(candidate, round.messages...)
		}
		after, countErr := countMessagesTokens(ctx, candidate, opts.counter, nil)
		if countErr != nil {
			break
		}
		if after <= opts.maxTokens {
			if opts.logger != nil {
				opts.logger.Warn("eino context compacted by dropping older rounds",
					zap.String("phase", opts.phase),
					zap.Int("tokens_before", before),
					zap.Int("tokens_after", after),
					zap.Int("max_tokens", opts.maxTokens),
					zap.Int("dropped_rounds", dropped),
					zap.Bool("aggressive", opts.aggressive),
				)
			}
			return candidate, true
		}
		if opts.aggressive {
			break
		}
	}

	if len(rounds) == 0 {
		return messages, false
	}
	latest := rounds[len(rounds)-1]
	truncated, truncErr := truncateRoundMessagesToTokenBudget(
		ctx, latest, opts.maxTokens, opts.counter, opts.toolMaxBytes, opts.spillRef,
	)
	if truncErr != nil || len(truncated) == 0 {
		if opts.logger != nil {
			opts.logger.Warn("eino context still above budget after round compaction; passing through without local error",
				zap.String("phase", opts.phase),
				zap.Int("tokens_before", before),
				zap.Int("max_tokens", opts.maxTokens),
				zap.Bool("aggressive", opts.aggressive),
			)
		}
		return messages, false
	}
	candidate := append([]adk.Message(nil), systems...)
	if dropped > 0 || startIdx > 0 {
		for _, round := range rounds[:len(rounds)-1] {
			candidate = append(candidate, round.messages...)
		}
	}
	candidate = append(candidate, truncated...)
	after, countErr := countMessagesTokens(ctx, candidate, opts.counter, nil)
	if countErr != nil {
		return messages, false
	}
	if opts.logger != nil {
		opts.logger.Warn("eino context compacted by truncating latest round tool output",
			zap.String("phase", opts.phase),
			zap.Int("tokens_before", before),
			zap.Int("tokens_after", after),
			zap.Int("max_tokens", opts.maxTokens),
			zap.Int("dropped_rounds", dropped),
			zap.Bool("aggressive", opts.aggressive),
		)
	}
	return candidate, true
}

func aggressiveCompactMessagesForOverflow(
	ctx context.Context,
	messages []adk.Message,
	maxTotalTokens int,
	modelName string,
	toolMaxBytes int,
	phase string,
	logger *zap.Logger,
) []adk.Message {
	if len(messages) == 0 || maxTotalTokens <= 0 {
		return messages
	}
	budget := maxTotalTokens * 70 / 100
	if budget < 4096 {
		budget = 4096
	}
	aggressiveToolMax := toolMaxBytes / aggressiveToolTruncDivisor
	if aggressiveToolMax < 2048 {
		aggressiveToolMax = 2048
	}
	out, _ := compactMessagesByDroppingRounds(ctx, messages, compactMessagesOpts{
		maxTokens:    budget,
		counter:      einoSummarizationTokenCounter(modelName),
		toolMaxBytes: aggressiveToolMax,
		aggressive:   true,
		logger:       logger,
		phase:        phase,
	})
	return out
}
