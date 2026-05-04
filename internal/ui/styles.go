package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Palette — dark-mode vibe coder aesthetic (Tokyo Night inspired).
var (
	colorPrimary   = lipgloss.Color("#7AA2F7")
	colorSecondary = lipgloss.Color("#A9B1D6")
	colorMuted     = lipgloss.Color("#565F89")
	colorSuccess   = lipgloss.Color("#9ECE6A")
	colorWarning   = lipgloss.Color("#E0AF68")
	colorError     = lipgloss.Color("#F7768E")
	colorBorder    = lipgloss.Color("#3B4261")
)

var (
	StyleTitle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	StyleSubtle = lipgloss.NewStyle().
			Foreground(colorMuted)

	StyleHighlight = lipgloss.NewStyle().
			Foreground(colorSecondary)

	StyleSuccess = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	StyleError = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	StyleWarning = lipgloss.NewStyle().
			Foreground(colorWarning)

	StyleDimmed = lipgloss.NewStyle().
			Foreground(colorMuted).
			Faint(true)

	StyleTag = lipgloss.NewStyle().
			Foreground(colorWarning).
			Background(lipgloss.Color("#2A2D3E")).
			Padding(0, 1)

	StyleBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	StyleLogEntry = lipgloss.NewStyle().
			Foreground(colorSecondary).
			PaddingLeft(2)

	StyleElapsed = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)
)

// PulseFrames are animation frames for the active indicator.
var PulseFrames = []string{"●", "◉", "○", "◉"}

func FormatDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	var text string
	if h > 0 {
		text = fmt.Sprintf("%dh %02dm %02ds", h, m, s)
	} else if m > 0 {
		text = fmt.Sprintf("%dm %02ds", m, s)
	} else {
		text = fmt.Sprintf("%ds", s)
	}
	return StyleElapsed.Render(text)
}
