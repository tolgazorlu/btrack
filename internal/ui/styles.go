package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

const (
	hexLightForeground          = "#1c4037"
	hexLightPrimary             = "#1fad91"
	hexLightAccent              = "#d9f2ec"
	hexLightAccentForeground    = "#0f5748"
	hexLightMuted               = "#e2f3ef"
	hexLightMutedForeground     = "#509584"
	hexLightSecondaryForeground = "#12493e"
	hexLightBorderOpaque        = "#94d1c5"

	hexDarkForeground          = "#d4ede8"
	hexDarkPrimary             = "#52e0c4"
	hexDarkAccent              = "#20323c"
	hexDarkAccentForeground    = "#93ecda"
	hexDarkMuted               = "#152228"
	hexDarkMutedForeground     = "#8fbcb3"
	hexDarkSecondaryForeground = "#d4ede8"
	hexDarkBorderOpaque        = "#367d6f"
)

func ac(light, dark string) lipgloss.AdaptiveColor {
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

var (
	colorForeground       = ac(hexLightForeground, hexDarkForeground)
	colorPrimary          = ac(hexLightPrimary, hexDarkPrimary)
	colorSecondary          = ac(hexLightSecondaryForeground, hexDarkSecondaryForeground)
	colorMuted              = ac(hexLightMutedForeground, hexDarkMutedForeground)
	colorAccent             = ac(hexLightAccent, hexDarkAccent)
	colorAccentForeground   = ac(hexLightAccentForeground, hexDarkAccentForeground)
	colorMutedFill          = ac(hexLightMuted, hexDarkMuted)
	colorBorder             = ac(hexLightBorderOpaque, hexDarkBorderOpaque)
	colorSuccess            = colorPrimary
	colorWarning            = colorAccentForeground
	colorError              = colorForeground
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
			Background(colorMutedFill).
			Bold(true).
			Padding(0, 1)

	StyleWarning = lipgloss.NewStyle().
			Foreground(colorWarning)

	StyleDimmed = lipgloss.NewStyle().
			Foreground(colorMuted).
			Faint(true)

	StyleTag = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Background(colorAccent).
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

var (
	ColorBorder    = colorBorder
	ColorMuted     = colorMuted
	ColorPrimary   = colorPrimary
	ColorSecondary = colorSecondary
)

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
