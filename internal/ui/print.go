package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const Indent = "  "

const RuleWidth = 56

const kvKeyWidth = 12

const cmdWidth = 36

var Out io.Writer = os.Stdout

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

func Blank() { fmt.Fprintln(Out) }

func Rule() { fmt.Fprintln(Out, Indent+ruleLine) }

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

func Section(label string) {
	fmt.Fprintln(Out, Indent+StyleDimmed.Render(label))
}

func OK(body string) { Sign(StyleSuccess.Render(Sym.OK), body) }

func Warn(body string) { Sign(StyleWarning.Render(Sym.Warn), body) }

func FailLine(body string) { Sign(StyleError.Render(Sym.Fail), body) }

func Sign(symbol, body string) {
	fmt.Fprintf(Out, "%s%s  %s\n", Indent, symbol, body)
}

func Hint(body string) {
	fmt.Fprintf(Out, "%s%s  %s\n", Indent, StyleDimmed.Render(Sym.Bullet), StyleDimmed.Render(body))
}

func Tip(body string) {
	fmt.Fprintf(Out, "%stip  %s\n", Indent, StyleDimmed.Render(body))
}

func Plain(body string) {
	fmt.Fprintf(Out, "%s%s\n", Indent, body)
}

func Dim(body string) {
	fmt.Fprintln(Out, Indent+StyleDimmed.Render(body))
}

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

func Footer(hint string) {
	Rule()
	if hint != "" {
		Hint(hint)
	}
	Blank()
}

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
