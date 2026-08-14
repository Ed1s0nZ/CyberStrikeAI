package c2

import (
	"encoding/base64"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func mustGBK(t *testing.T, s string) []byte {
	t.Helper()
	out, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestNormalizeConsoleOutput_WindowsGBK(t *testing.T) {
	raw := mustGBK(t, "中文测试")
	got := NormalizeConsoleOutput(raw, "windows")
	if got != "中文测试" {
		t.Fatalf("got %q want 中文测试", got)
	}
}

func TestNormalizeConsoleOutput_UTF8Passthrough(t *testing.T) {
	raw := []byte("hello 世界")
	got := NormalizeConsoleOutput(raw, "linux")
	if got != "hello 世界" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveTaskResultText_PrefersB64(t *testing.T) {
	raw := mustGBK(t, "采购订单")
	b64 := base64.StdEncoding.EncodeToString(raw)
	got := ResolveTaskResultText("", b64, "windows")
	if got != "采购订单" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveTaskResultText_PlainFallback(t *testing.T) {
	raw := mustGBK(t, "测试")
	got := ResolveTaskResultText(string(raw), "", "windows")
	if got != "测试" {
		t.Fatalf("got %q", got)
	}
}
