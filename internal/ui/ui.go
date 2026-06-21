// Package ui provides minimal, dependency-free colored terminal output for the CLI.
package ui

import (
	"fmt"
	"os"
)

var colorEnabled = isTTY(os.Stdout)

// ANSI escape codes.
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
)

// DisableColor turns off ANSI coloring (used by --no-color and non-TTY output).
func DisableColor() { colorEnabled = false }

func paint(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + reset
}

// Banner returns the togo wordmark shown in help output.
func Banner() string {
	return paint(cyan+bold, "  ▌ togo") + paint(gray, " — Go, the artisan way") + "\n"
}

// Success prints a green check line.
func Success(format string, a ...any) {
	fmt.Fprintln(os.Stdout, paint(green, "✓ ")+fmt.Sprintf(format, a...))
}

// Info prints a neutral informational line.
func Info(format string, a ...any) {
	fmt.Fprintln(os.Stdout, paint(blue, "→ ")+fmt.Sprintf(format, a...))
}

// Warn prints a yellow warning line.
func Warn(format string, a ...any) {
	fmt.Fprintln(os.Stderr, paint(yellow, "! ")+fmt.Sprintf(format, a...))
}

// Error prints a red error line.
func Error(format string, a ...any) {
	fmt.Fprintln(os.Stderr, paint(red+bold, "✗ ")+fmt.Sprintf(format, a...))
}

// Step prints a dimmed sub-step line, used by the generator and orchestrator.
func Step(format string, a ...any) {
	fmt.Fprintln(os.Stdout, paint(dim, "  "+fmt.Sprintf(format, a...)))
}

// Label colorizes a short status tag like CREATE / SKIP / OVERWRITE.
func Label(kind string) string {
	switch kind {
	case "CREATE", "APPEND":
		return paint(green+bold, kind)
	case "OVERWRITE", "FORCE":
		return paint(yellow+bold, kind)
	case "SKIP":
		return paint(gray, kind)
	case "DELETE":
		return paint(red+bold, kind)
	default:
		return paint(cyan, kind)
	}
}

func isTTY(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
