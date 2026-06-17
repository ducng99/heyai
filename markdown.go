package main

import (
	"os"

	"charm.land/glamour/v2"
	"golang.org/x/term"
)

const defaultTerminalWidth = 100

func newTerminalMarkdownRenderer(out *os.File) (*glamour.TermRenderer, error) {
	width := defaultTerminalWidth
	if detectedWidth, _, err := term.GetSize(int(out.Fd())); err == nil && detectedWidth > 0 {
		width = detectedWidth
	}
	return glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(width),
	)
}

func isTerminal(out *os.File) bool {
	return term.IsTerminal(int(out.Fd()))
}
