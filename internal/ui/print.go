package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Indent is the 2-space leading gutter every printed line uses.
const Indent = "  "

// RuleWidth is the width of horizontal rules and the layout target.
const RuleWidth = 56

// kvKeyWidth is the fixed column width for KV row keys.
const kvKeyWidth = 12

// cmdWidth is the fixed column width for command lists (Cmd helper).
const cmdWidth = 36

// Out is the writer Print* helpers target. Tests may redirect it.
var Out io.Writer = os.Stdout

// Sym is the canonical symbol set for btrack output. Use it instead of inline glyphs.
var Sym = struct {
	Start, Stop, Pause, Switch, Resume string
	OK, Fail, Warn, Bullet, Arrow      string
	Branch, Up                         string
}{
	Start: "▸", Stop: "◼", Pause: "⏸", Switch: "⇄", Resume: "↻",
	OK: "✓", Fail: "✗", Warn: "!", Bullet: "·", Arrow: "→",
	Branch: "⎇", Up: "↑",
}

var ruleLine = StyleDimmed.Render(strings.Repeat("─", RuleWidth))

// Blank prints a single empty line.
func Blank() { fmt.Fprintln(Out) }

// Rule prints a thin dim horizontal rule at the standard indent.
func Rule() { fmt.Fprintln(Out, Indent+ruleLine) }

// Header prints the canonical command heading: a blank line, then
// "  btrack <name>  <subtitle>" with the standard rule beneath it.
// Pass empty strings to skip a part.
func Header(name, subtitle string) {
	line := Indent + StyleTitle.Render("btrack")
	if name != "" {
		line += " " + StyleHighlight.Render(name)
	}
	if subtitle != "" {
		line += "  " + StyleDimmed.Render(subtitle)
	}
	Blank()
	fmt.Fprintln(Out, line)
	Rule()
}

// Section prints a small dim section label, lower-cased.
func Section(label string) {
	fmt.Fprintln(Out, Indent+StyleDimmed.Render(label))
}

// OK prints "  ✓  body".
func OK(body string) { Sign(StyleSuccess.Render(Sym.OK), body) }

// Warn prints "  !  body".
func Warn(body string) { Sign(StyleWarning.Render(Sym.Warn), body) }

// FailLine prints "  ✗  body" (named to avoid shadowing testing.T.Fail conventions).
func FailLine(body string) { Sign(StyleError.Render(Sym.Fail), body) }

// Sign prints "  <symbol>  body" with the standard indent.
func Sign(symbol, body string) {
	fmt.Fprintf(Out, "%s%s  %s\n", Indent, symbol, body)
}

// Hint prints a fully-dimmed "  · body" line. Use for trailing tips.
func Hint(body string) {
	fmt.Fprintf(Out, "%s%s  %s\n", Indent, StyleDimmed.Render(Sym.Bullet), StyleDimmed.Render(body))
}

// Tip prints "  tip  body" (label + dim body).
func Tip(body string) {
	fmt.Fprintf(Out, "%stip  %s\n", Indent, StyleDimmed.Render(body))
}

// Plain prints a body line at the standard indent (no symbol, no styling).
func Plain(body string) {
	fmt.Fprintf(Out, "%s%s\n", Indent, body)
}

// Dim prints "  body" with the body styled dim.
func Dim(body string) {
	fmt.Fprintln(Out, Indent+StyleDimmed.Render(body))
}

// KV prints "  key  value" with key padded to a fixed column.
// key is rendered dim; value is passed through (caller may style it).
func KV(key, value string) {
	pad := kvKeyWidth - lipglossLen(key)
	if pad < 1 {
		pad = 1
	}
	fmt.Fprintf(Out, "%s%s%s  %s\n",
		Indent,
		StyleDimmed.Render(key),
		strings.Repeat(" ", pad),
		value,
	)
}

// Cmd prints a help-style "  command   description" row.
func Cmd(cmd, desc string) {
	pad := cmdWidth - len(cmd)
	if pad < 1 {
		pad = 1
	}
	fmt.Fprintf(Out, "%s%s%s%s\n",
		Indent,
		StyleHighlight.Render(cmd),
		strings.Repeat(" ", pad),
		StyleDimmed.Render(desc),
	)
}

// Footer prints a closing rule + a dim hint line + a trailing blank.
// Pass empty hint to skip the hint line.
func Footer(hint string) {
	Rule()
	if hint != "" {
		Hint(hint)
	}
	Blank()
}

// lipglossLen counts visible runes ignoring ANSI escapes.
func lipglossLen(s string) int {
	n := 0
	in := false
	for _, r := range s {
		if r == 0x1b {
			in = true
			continue
		}
		if in {
			if r == 'm' {
				in = false
			}
			continue
		}
		n++
	}
	return n
}
