package multiagent

import (
	"strings"
	"testing"
)

// Eino execute 去重分支 EOF flush 须以 mainAssistantBuf 为基准计算 tail，
// 若误用 TrimSpace(mainAssistantBuf)，会与已推前缀在空白处失配，normalize 走拼接路径叠字。
func TestNormalizeStreamingDelta_eofTailUsesRawBufNotTrim(t *testing.T) {
	wireAccum := "phrase "
	rawFull := "phrase \n"
	_, tail := normalizeStreamingDelta(wireAccum, rawFull)
	if want := "\n"; tail != want {
		t.Fatalf("tail=%q want %q", tail, want)
	}

	nextWrong, badTail := normalizeStreamingDelta(wireAccum, strings.TrimSpace(rawFull))
	if badTail != "phrase" || nextWrong != "phrase phrase" {
		t.Fatalf("trimmed full vs wire prefix mismatch should concat-append; got next=%q badTail=%q", nextWrong, badTail)
	}
}
