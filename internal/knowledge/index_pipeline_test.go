package knowledge

import "testing"

func TestNormalizeChunkStrategy(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "markdown_then_recursive"},
		{"recursive", "recursive"},
		{"RECURSIVE", "recursive"},
		{"markdown_then_recursive", "markdown_then_recursive"},
		{"markdown", "markdown_then_recursive"},
		{"unknown", "markdown_then_recursive"},
	}
	for _, tc := range cases {
		if got := normalizeChunkStrategy(tc.in); got != tc.want {
			t.Errorf("normalizeChunkStrategy(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
