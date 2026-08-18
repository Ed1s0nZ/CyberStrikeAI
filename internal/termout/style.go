package termout

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	codeReset  = "\033[0m"
	codeBold   = "\033[1m"
	codeDim    = "\033[2m"
	codeRed    = "\033[31m"
	codeGreen  = "\033[32m"
	codeYellow = "\033[33m"
	codeBlue   = "\033[34m"
	codeCyan   = "\033[36m"
	codeWhite  = "\033[97m"
)

// Style wraps ANSI styling with TTY / NO_COLOR awareness.
type Style struct {
	out     io.Writer
	enabled bool
}

// New creates a Style writing to out (typically os.Stdout).
func New(out io.Writer) *Style {
	return &Style{out: out, enabled: colorEnabled(out)}
}

func colorEnabled(w io.Writer) bool {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	force := strings.TrimSpace(os.Getenv("FORCE_COLOR"))
	if force == "1" || strings.EqualFold(force, "true") || strings.EqualFold(force, "yes") {
		return true
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func (s *Style) paint(code, text string) string {
	if !s.enabled || text == "" {
		return text
	}
	return code + text + codeReset
}

func (s *Style) Bold(text string) string   { return s.paint(codeBold, text) }
func (s *Style) Dim(text string) string    { return s.paint(codeDim, text) }
func (s *Style) Red(text string) string    { return s.paint(codeRed, text) }
func (s *Style) Green(text string) string  { return s.paint(codeGreen, text) }
func (s *Style) Yellow(text string) string { return s.paint(codeYellow, text) }
func (s *Style) Blue(text string) string   { return s.paint(codeBlue, text) }
func (s *Style) Cyan(text string) string   { return s.paint(codeCyan, text) }
func (s *Style) White(text string) string  { return s.paint(codeWhite, text) }

func (s *Style) Println(text string) {
	_, _ = fmt.Fprintln(s.out, text)
}

func (s *Style) Printf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(s.out, format, args...)
}

func (s *Style) BlankLine() {
	s.Println("")
}

func (s *Style) boxTop(innerWidth int) string {
	return s.Cyan("╭" + strings.Repeat("─", innerWidth+2) + "╮")
}

func (s *Style) boxBottom(innerWidth int) string {
	return s.Cyan("╰" + strings.Repeat("─", innerWidth+2) + "╯")
}

func (s *Style) boxRow(innerWidth int, content string) string {
	return s.Cyan("│ ") + padRightDisplay(content, innerWidth) + s.Cyan(" │")
}

func (s *Style) printBox(rows []string, minInner, maxInner int) {
	inner := maxDisplayWidth(rows...)
	if inner < minInner {
		inner = minInner
	}
	if maxInner > 0 && inner > maxInner {
		inner = maxInner
	}

	s.BlankLine()
	s.Println(s.boxTop(inner))
	for _, row := range rows {
		s.Println(s.boxRow(inner, row))
	}
	s.Println(s.boxBottom(inner))
	s.BlankLine()
}
