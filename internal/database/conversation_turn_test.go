package database

import (
	"testing"
)

func TestTurnSliceRange(t *testing.T) {
	mk := func(id, role string) Message {
		return Message{ID: id, Role: role}
	}
	msgs := []Message{
		mk("u1", "user"),
		mk("a1", "assistant"),
		mk("u2", "user"),
		mk("a2", "assistant"),
	}
	cases := []struct {
		anchor string
		start  int
		end    int
	}{
		{"u1", 0, 2},
		{"a1", 0, 2},
		{"u2", 2, 4},
		{"a2", 2, 4},
	}
	for _, tc := range cases {
		s, e, err := turnSliceRange(msgs, tc.anchor)
		if err != nil {
			t.Fatalf("anchor %s: %v", tc.anchor, err)
		}
		if s != tc.start || e != tc.end {
			t.Fatalf("anchor %s: got [%d,%d) want [%d,%d)", tc.anchor, s, e, tc.start, tc.end)
		}
	}
	if _, _, err := turnSliceRange(msgs, "nope"); err == nil {
		t.Fatal("expected error for missing id")
	}
}
