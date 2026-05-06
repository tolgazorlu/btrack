package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestConsoleBoxLeftEdgeAligned(t *testing.T) {
	m := NewConsoleModel("", "", "", false)
	tm, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	out := tm.(ConsoleModel).View()

	var top, mid, bot int = -1, -1, -1
	for i, line := range strings.Split(out, "\n") {
		switch {
		case strings.Contains(line, "╭"):
			top = strings.Index(line, "╭")
		case strings.Contains(line, "╰"):
			bot = strings.Index(line, "╰")
		case strings.Contains(line, "│ ") && mid < 0:
			mid = strings.Index(line, "│")
			_ = i
		}
	}

	if top < 0 || mid < 0 || bot < 0 {
		t.Fatalf("missing border rows: top=%d mid=%d bot=%d\n%s", top, mid, bot, out)
	}
	if top != mid || mid != bot {
		t.Errorf("box left edges misaligned: top=%d mid=%d bot=%d", top, mid, bot)
	}
}
