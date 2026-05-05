package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Palette — docs Emerald fd-* theme (@theme + .dark), hex from the same HSL values.
// lipgloss.AdaptiveColor: Light = @theme (light UI), Dark = .dark
//
// Docs also define fd-background, fd-primary-foreground, fd-secondary, fd-card,
// fd-popover, fd-ring (same hue as primary) — available for future full-bleed UI;
// this file maps the tokens used by btrack TUI and plain-text helpers.
const (
	// @theme (light) — hex from docs fd-* HSL
	hexLightForeground          = "#1c4037" // hsl(165, 40%, 18%)  fd-foreground
	hexLightPrimary             = "#1fad91" // hsl(168, 70%, 40%)  fd-primary
	hexLightAccent              = "#d9f2ec" // hsl(165, 50%, 90%)  fd-accent
	hexLightAccentForeground    = "#0f5748" // hsl(168, 70%, 20%)  fd-accent-foreground
	hexLightMuted               = "#e2f3ef" // hsl(165, 40%, 92%)  fd-muted
	hexLightMutedForeground     = "#509584" // hsl(165, 30%, 45%)  fd-muted-foreground
	hexLightSecondaryForeground = "#12493e" // hsl(168, 60%, 18%)  fd-secondary-foreground
	hexLightBorderOpaque        = "#94d1c5" // hsl(168, 40%, 70%)  fd-border (opaque TUI stand-in)

	// .dark
	hexDarkForeground          = "#d4ede8" // hsl(168, 40%, 88%)  fd-foreground
	hexDarkPrimary             = "#52e0c4" // hsl(168, 70%, 60%)  fd-primary
	hexDarkAccent              = "#20323c" // hsl(200, 30%, 18%)  fd-accent
	hexDarkAccentForeground    = "#93ecda" // hsl(168, 70%, 75%)  fd-accent-foreground
	hexDarkMuted               = "#152228" // hsl(200, 30%, 12%)  fd-muted
	hexDarkMutedForeground     = "#8fbcb3" // hsl(168, 25%, 65%)  fd-muted-foreground
	hexDarkSecondaryForeground = "#d4ede8" // hsl(168, 40%, 88%)  fd-secondary-foreground
	hexDarkBorderOpaque        = "#367d6f" // hsl(168, 40%, 35%)  fd-border (opaque TUI stand-in)
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
	colorSuccess            = colorPrimary // completed / positive — fd-primary
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

// ColorBorder, ColorMuted, ColorPrimary, ColorSecondary — for cmd-local lipgloss
// (tables, welcome). Other fd tokens drive Style* above and the hex const block.
var (
	ColorBorder    = colorBorder
	ColorMuted     = colorMuted
	ColorPrimary   = colorPrimary
	ColorSecondary = colorSecondary
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
