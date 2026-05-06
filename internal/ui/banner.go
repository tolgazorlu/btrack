package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// bannerLines holds the pixel-block "btrack" ASCII art.
// ANSI Shadow font, 6 rows tall — designed to mirror the Gemini CLI welcome aesthetic.
var bannerLines = []string{
	"  ██████╗ ████████╗██████╗  █████╗  ██████╗██╗  ██╗",
	"  ██╔══██╗╚══██╔══╝██╔══██╗██╔══██╗██╔════╝██║ ██╔╝",
	"  ██████╔╝   ██║   ██████╔╝███████║██║     █████╔╝ ",
	"  ██╔══██╗   ██║   ██╔══██╗██╔══██║██║     ██╔═██╗ ",
	"  ██████╔╝   ██║   ██║  ██║██║  ██║╚██████╗██║  ██╗",
	"  ╚═════╝    ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝╚═╝  ╚═╝",
}

// gradient hex stops, primary → accent foreground (light & dark variants).
var bannerStops = []struct{ light, dark string }{
	{"#1fad91", "#52e0c4"}, // primary
	{"#1aa28a", "#5ae0c8"},
	{"#129682", "#6cdcd0"},
	{"#0f7d75", "#7ed0d8"},
	{"#0c6b75", "#8fc4dc"},
	{"#0f5748", "#93ecda"}, // accent foreground
}

// Banner returns the multi-line, gradient-styled "btrack" banner.
func Banner() string {
	var sb strings.Builder
	for i, line := range bannerLines {
		stop := bannerStops[i%len(bannerStops)]
		style := lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Light: stop.light, Dark: stop.dark}).
			Bold(true)
		sb.WriteString(style.Render(line))
		sb.WriteString("\n")
	}
	return sb.String()
}

// PrintBanner writes the gradient banner to Out followed by a tagline.
func PrintBanner(version, tagline string) {
	Blank()
	fmt.Fprint(Out, Banner())
	Blank()
	if tagline == "" {
		tagline = "time tracker for developers"
	}
	line := "  " + StyleDimmed.Render(tagline)
	if version != "" {
		line += "  " + StyleDimmed.Render("·") + "  " + StyleHighlight.Render(version)
	}
	fmt.Fprintln(Out, line)
	Blank()
}
