// Package pricing provides an embedded, source-cited table of LLM token
// prices and a cost estimator built on top of it.
package pricing

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed prices.json
var pricesJSON []byte

// Tokens holds a token usage breakdown for a single request or session.
// Input is the uncached input token count.
type Tokens struct {
	Input      int64
	Output     int64
	CacheRead  int64
	CacheWrite int64
}

// Price holds USD-per-1M-token rates for a model.
type Price struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

type modelEntry struct {
	ID         string  `json:"id"`
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
	Source     string  `json:"source"`
}

type priceTable struct {
	CheckedAt string       `json:"checked_at"`
	Models    []modelEntry `json:"models"`
}

var (
	table     priceTable
	byID      map[string]Price
	checkedAt string
)

func init() {
	if err := json.Unmarshal(pricesJSON, &table); err != nil {
		panic("pricing: failed to parse embedded prices.json: " + err.Error())
	}
	checkedAt = table.CheckedAt
	byID = make(map[string]Price, len(table.Models))
	for _, m := range table.Models {
		byID[normalize(m.ID)] = Price{
			Input:      m.Input,
			Output:     m.Output,
			CacheRead:  m.CacheRead,
			CacheWrite: m.CacheWrite,
		}
	}
}

// CheckedAt returns the date the embedded price table was last verified
// against official vendor pricing pages.
func CheckedAt() string {
	return checkedAt
}

// knownPrefixes are provider prefixes stripped before matching, e.g.
// "anthropic/claude-sonnet-4-5" -> "claude-sonnet-4-5".
var knownPrefixes = []string{
	"anthropic/",
	"openai/",
	"google/",
	"gemini/",
	"vertex_ai/",
	"vertex/",
	"bedrock/",
	"azure/",
}

// normalize lowercases a model id, strips known provider prefixes, and
// treats '.' and '-' interchangeably so version suffixes compare equal
// regardless of separator (e.g. "claude-sonnet-4.5" == "claude-sonnet-4-5").
// It does not strip date suffixes; that is handled separately by callers
// that need prefix matching.
func normalize(model string) string {
	s := strings.ToLower(strings.TrimSpace(model))
	for _, p := range knownPrefixes {
		if strings.HasPrefix(s, p) {
			s = strings.TrimPrefix(s, p)
			break
		}
	}
	// Any remaining provider-style prefix separated by '/': keep the last
	// path segment.
	if idx := strings.LastIndex(s, "/"); idx != -1 {
		s = s[idx+1:]
	}
	s = strings.ReplaceAll(s, ".", "-")
	return s
}

// isDateSuffix reports whether tok looks like a date suffix component, e.g.
// "20250514" (8 digits) found at the end of a normalized model id.
func stripDateSuffix(s string) string {
	// Strip a trailing "-YYYYMMDD" (8 digit) component, if present.
	idx := strings.LastIndex(s, "-")
	if idx == -1 {
		return s
	}
	tail := s[idx+1:]
	if len(tail) == 8 && isAllDigits(tail) {
		return s[:idx]
	}
	return s
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isLocalModel reports whether model refers to a local/self-hosted model,
// e.g. "qwen3.8:27b" or any name mentioning ollama. Such models are free to
// run locally and are always known with a zero price.
func isLocalModel(model string) bool {
	lower := strings.ToLower(model)
	return strings.Contains(model, ":") || strings.Contains(lower, "ollama")
}

// Lookup returns the USD-per-1M-token price for model, and whether it was
// found. Matching normalises the id (lowercase, strips provider prefixes,
// strips date suffixes, treats '.' and '-' as equivalent in versions), then
// tries an exact match followed by a longest-prefix match against known
// model ids. Local/self-hosted models (ids containing ':' or mentioning
// "ollama") are always known, with a zero price.
func Lookup(model string) (Price, bool) {
	if isLocalModel(model) {
		return Price{}, true
	}

	norm := normalize(model)

	if p, ok := byID[norm]; ok {
		return p, true
	}

	stripped := stripDateSuffix(norm)
	if stripped != norm {
		if p, ok := byID[stripped]; ok {
			return p, true
		}
	}

	// Longest-prefix match: find the known id that is the longest prefix of
	// the (date-stripped) normalized query, matching on a '-' boundary so
	// "claude-sonnet-4" does not match a query like "claude-sonnet-45".
	candidate := stripped
	var best string
	for id := range byID {
		if id == candidate {
			continue
		}
		if strings.HasPrefix(candidate, id) {
			rest := candidate[len(id):]
			if rest == "" || strings.HasPrefix(rest, "-") {
				if len(id) > len(best) {
					best = id
				}
			}
		}
	}
	if best != "" {
		return byID[best], true
	}

	return Price{}, false
}

// Estimate computes the USD cost of the given token usage for model. known
// is false if the model's price could not be determined, in which case usd
// is 0.
func Estimate(model string, t Tokens) (usd float64, known bool) {
	p, ok := Lookup(model)
	if !ok {
		return 0, false
	}
	const perToken = 1.0 / 1_000_000.0
	usd = float64(t.Input)*p.Input*perToken +
		float64(t.Output)*p.Output*perToken +
		float64(t.CacheRead)*p.CacheRead*perToken +
		float64(t.CacheWrite)*p.CacheWrite*perToken
	return usd, true
}
