package handler

import (
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// mustEncode 使用指定编码对 UTF-8 字符串做编码，得到原始字节，用于构造测试输入
func mustEncode(t *testing.T, s string, enc string) []byte {
	t.Helper()
	var tr transform.Transformer
	switch enc {
	case "gbk":
		tr = simplifiedchinese.GBK.NewEncoder()
	case "gb18030":
		tr = simplifiedchinese.GB18030.NewEncoder()
	default:
		t.Fatalf("unsupported test encoding: %s", enc)
	}
	out, _, err := transform.Bytes(tr, []byte(s))
	if err != nil {
		t.Fatalf("mustEncode(%s) failed: %v", enc, err)
	}
	return out
}

func TestNormalizeWebshellEncoding(t *testing.T) {
	cases := map[string]string{
		"":         "auto",
		"   ":      "auto",
		"auto":     "auto",
		"AUTO":     "auto",
		"utf-8":    "utf-8",
		"UTF-8":    "utf-8",
		"utf8":     "utf-8",
		"gbk":      "gbk",
		"GBK":      "gbk",
		"gb18030":  "gb18030",
		"big5":     "auto", // 未支持的回退到 auto
		"anything": "auto",
	}
	for in, want := range cases {
		if got := normalizeWebshellEncoding(in); got != want {
			t.Errorf("normalizeWebshellEncoding(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDecodeWebshellOutput_AutoDetectsGBK(t *testing.T) {
	// 模拟 Windows 中文 cmd 输出的 GBK 字节流
	want := "用户名                        SID                                            类型"
	raw := mustEncode(t, want, "gbk")

	// auto 模式：UTF-8 校验失败后应当回退 GB18030 解码，得到原始中文
	got := decodeWebshellOutput(raw, "auto")
	if got != want {
		t.Errorf("decodeWebshellOutput(auto) = %q, want %q", got, want)
	}

	// 显式 GBK 模式：同样应当正确解码
	got = decodeWebshellOutput(raw, "gbk")
	if got != want {
		t.Errorf("decodeWebshellOutput(gbk) = %q, want %q", got, want)
	}

	// 显式 GB18030 模式：GBK 是 GB18030 子集，也应正确解码
	got = decodeWebshellOutput(raw, "gb18030")
	if got != want {
		t.Errorf("decodeWebshellOutput(gb18030) = %q, want %q", got, want)
	}
}

func TestDecodeWebshellOutput_PassthroughUTF8(t *testing.T) {
	// 已经是 UTF-8 的中文字符串，各模式都应返回原串（不破坏）
	want := "hello 世界"
	for _, enc := range []string{"", "auto", "utf-8"} {
		if got := decodeWebshellOutput([]byte(want), enc); got != want {
			t.Errorf("decodeWebshellOutput(%q) passthrough = %q, want %q", enc, got, want)
		}
	}
}

func TestDecodeWebshellOutput_ASCIIStable(t *testing.T) {
	// 纯 ASCII 在任何模式下都必须保持原样
	want := "whoami\nAdministrator\n"
	for _, enc := range []string{"", "auto", "utf-8", "gbk", "gb18030"} {
		if got := decodeWebshellOutput([]byte(want), enc); got != want {
			t.Errorf("decodeWebshellOutput(%q) ASCII = %q, want %q", enc, got, want)
		}
	}
}

func TestDecodeWebshellOutput_EmptyInput(t *testing.T) {
	// 空输入直接返回空串，不做额外分配
	if got := decodeWebshellOutput(nil, "gbk"); got != "" {
		t.Errorf("decodeWebshellOutput(nil) = %q, want empty", got)
	}
	if got := decodeWebshellOutput([]byte{}, "auto"); got != "" {
		t.Errorf("decodeWebshellOutput([]) = %q, want empty", got)
	}
}
