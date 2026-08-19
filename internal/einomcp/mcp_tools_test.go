package einomcp

import (
	"strings"
	"testing"
)

func TestUnknownToolReminderText(t *testing.T) {
	s := unknownToolReminderText("bad_tool")
	if !strings.Contains(s, "bad_tool") {
		t.Fatalf("expected requested name in message: %s", s)
	}
	if strings.Contains(s, "Tools currently available") {
		t.Fatal("unified message must not list tool names")
	}
}

func TestSanitizeOpenAIToolName(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"fs.read", "fs_read"},
		{"server.exec", "server_exec"},
		{"meta.whoami", "meta_whoami"},
		{"nezha::fs.read", "nezha__fs_read"},
		{"simple_tool", "simple_tool"},
		{"with-dash", "with-dash"},
		{"spaces and 中文", "spaces_and___"},
		{"", ""},
	}
	for _, c := range cases {
		if got := sanitizeOpenAIToolName(c.in); got != c.want {
			t.Errorf("sanitizeOpenAIToolName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
