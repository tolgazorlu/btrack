package ingest

import (
	"regexp"
	"strings"
)

// absPathRe matches absolute Unix paths with at least two segments, as
// they appear inside JSON tool arguments and shell commands.
var absPathRe = regexp.MustCompile(`/(?:Users|home)/[A-Za-z0-9._-]+/[^\s"'\\,;:)}\]]+`)

// maxHintsPerSession bounds memory for very long sessions; the dominant
// project is clear long before this many paths.
const maxHintsPerSession = 2000

// collectPathHints appends absolute paths found in text to hints.
func collectPathHints(hints []string, text string) []string {
	if len(hints) >= maxHintsPerSession || !strings.Contains(text, "/") {
		return hints
	}
	for _, m := range absPathRe.FindAllString(text, -1) {
		hints = append(hints, strings.TrimRight(m, "./"))
		if len(hints) >= maxHintsPerSession {
			break
		}
	}
	return hints
}
