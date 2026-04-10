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
