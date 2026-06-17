package main

import (
	"os"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

const defaultTerminalWidth = 100

func newTerminalMarkdownRenderer(out *os.File) (*glamour.TermRenderer, error) {
	width := defaultTerminalWidth
	if detectedWidth, _, err := term.GetSize(int(out.Fd())); err == nil && detectedWidth > 0 {
		width = detectedWidth
	}
	return glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
}

func isTerminal(out *os.File) bool {
	return term.IsTerminal(int(out.Fd()))
}
