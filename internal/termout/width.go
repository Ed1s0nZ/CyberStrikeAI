package termout

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/width"
)

var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// displayWidth returns the terminal display width of text, ignoring ANSI codes.
func displayWidth(text string) int {
	plain := ansiEscapeRe.ReplaceAllString(text, "")
	w := 0
	for _, r := range plain {
		w += runeDisplayWidth(r)
	}
	return w
}

func runeDisplayWidth(r rune) int {
	if r == utf8.RuneError {
		return 0
	}
	// Most emoji / symbols render as double-width in modern terminals.
	if isEmojiLikeRune(r) {
		return 2
	}
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}

func isEmojiLikeRune(r rune) bool {
	switch {
	case r >= 0x1F300 && r <= 0x1FAFF: // pictographs / emoji
		return true
	case r >= 0x2600 && r <= 0x27BF: // misc symbols
		return true
	case r >= 0x2300 && r <= 0x23FF: // misc technical (⌚ etc.)
		return true
	case r >= 0x2B50 && r <= 0x2B55:
		return true
	default:
		return false
	}
}

func padRightDisplay(text string, target int) string {
	if target <= 0 {
		return ""
	}
	gap := target - displayWidth(text)
	if gap <= 0 {
		return text
	}
	return text + strings.Repeat(" ", gap)
}

func maxDisplayWidth(rows ...string) int {
	max := 0
	for _, row := range rows {
		if w := displayWidth(row); w > max {
			max = w
		}
	}
	return max
}
