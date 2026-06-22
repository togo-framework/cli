package ui

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// Option is a selectable item in a prompt.
type Option struct {
	Value   string
	Label   string
	Hint    string
	Default bool
}

// cyan/dim helpers reuse the package palette.
func pcyan(s string) string  { return paint(cyan, s) }
func pgreen(s string) string { return paint(green, s) }
func pdim(s string) string   { return paint(dim, s) }
func pbold(s string) string  { return paint(bold, s) }

// MultiSelect renders a Laravel-Prompts-style checkbox list. Space toggles, ↑/↓
// move, 'a' toggles all, Enter confirms. Falls back to the defaults when stdin is
// not a terminal. Returns the chosen values.
func MultiSelect(label string, opts []Option) []string {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return defaults(opts)
	}
	state, err := term.MakeRaw(fd)
	if err != nil {
		return defaults(opts)
	}
	defer term.Restore(fd, state)

	sel := make([]bool, len(opts))
	for i, o := range opts {
		sel[i] = o.Default
	}
	cursor := 0
	buf := make([]byte, 3)
	rendered := 0

	for {
		rendered = renderMulti(label, opts, sel, cursor, rendered)
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			continue
		}
		switch {
		case buf[0] == 3 || buf[0] == 'q': // Ctrl-C / q → cancel = keep defaults
			term.Restore(fd, state)
			fmt.Print("\r\n")
			return defaults(opts)
		case buf[0] == '\r' || buf[0] == '\n':
			term.Restore(fd, state)
			fmt.Print("\r\n")
			return chosen(opts, sel)
		case buf[0] == ' ':
			sel[cursor] = !sel[cursor]
		case buf[0] == 'a':
			all := !allTrue(sel)
			for i := range sel {
				sel[i] = all
			}
		case n >= 3 && buf[0] == 27 && buf[1] == '[':
			if buf[2] == 'A' { // up
				cursor = (cursor - 1 + len(opts)) % len(opts)
			} else if buf[2] == 'B' { // down
				cursor = (cursor + 1) % len(opts)
			}
		}
	}
}

// renderMulti redraws the list in place and returns the number of lines drawn.
func renderMulti(label string, opts []Option, sel []bool, cursor, prev int) int {
	if prev > 0 {
		fmt.Printf("\x1b[%dA", prev) // move cursor up
	}
	var b strings.Builder
	clr := "\x1b[2K"
	fmt.Fprintf(&b, "%s%s %s\r\n", clr, pcyan("◆"), pbold(label))
	for i, o := range opts {
		box := pdim("◻")
		if sel[i] {
			box = pgreen("◼")
		}
		ptr := "  "
		line := o.Label
		if i == cursor {
			ptr = pcyan("›") + " "
			line = pbold(o.Label)
		}
		hint := ""
		if o.Hint != "" {
			hint = "  " + pdim(o.Hint)
		}
		fmt.Fprintf(&b, "%s%s%s %s%s\r\n", clr, ptr, box, line, hint)
	}
	fmt.Fprintf(&b, "%s%s\r\n", clr, pdim("↑/↓ move · space toggle · a all · enter confirm"))
	fmt.Print(b.String())
	return len(opts) + 2
}

// Confirm asks a yes/no question (Enter accepts the default).
func Confirm(label string, def bool) bool {
	fd := int(os.Stdin.Fd())
	suffix := "[y/N]"
	if def {
		suffix = "[Y/n]"
	}
	if !term.IsTerminal(fd) {
		return def
	}
	fmt.Printf("%s %s %s ", pcyan("◆"), pbold(label), pdim(suffix))
	var line string
	fmt.Scanln(&line)
	line = strings.ToLower(strings.TrimSpace(line))
	if line == "" {
		return def
	}
	return line == "y" || line == "yes"
}

func defaults(opts []Option) []string {
	var out []string
	for _, o := range opts {
		if o.Default {
			out = append(out, o.Value)
		}
	}
	return out
}

func chosen(opts []Option, sel []bool) []string {
	var out []string
	for i, o := range opts {
		if sel[i] {
			out = append(out, o.Value)
		}
	}
	return out
}

func allTrue(s []bool) bool {
	for _, v := range s {
		if !v {
			return false
		}
	}
	return true
}
