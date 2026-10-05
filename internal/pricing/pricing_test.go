package pricing

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestCheckedAt(t *testing.T) {
	if CheckedAt() == "" {
		t.Fatal("CheckedAt() returned empty string")
	}
}

func TestLookupExactMatch(t *testing.T) {
	p, ok := Lookup("claude-sonnet-4-5")
	if !ok {
		t.Fatal("expected claude-sonnet-4-5 to be known")
	}
	if p.Input != 3.0 || p.Output != 15.0 {
		t.Fatalf("unexpected price: %+v", p)
	}
}

func TestLookupNormalization(t *testing.T) {
	cases := []struct {
		name  string
		model string
	}{
		{"lowercase", "CLAUDE-SONNET-4-5"},
		{"provider prefix anthropic", "anthropic/claude-sonnet-4-5"},
		{"dot vs dash version", "claude-sonnet-4.5"},
		{"dot version with prefix", "anthropic/claude-sonnet-4.5"},
		{"date suffix", "claude-sonnet-4-5-20250929"},
		{"dot version with date suffix", "claude-sonnet-4.5-20250929"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, ok := Lookup(c.model)
			if !ok {
				t.Fatalf("expected %q to be known", c.model)
			}
			if p.Input != 3.0 || p.Output != 15.0 {
				t.Fatalf("unexpected price for %q: %+v", c.model, p)
			}
		})
	}
}

func TestLookupOpenAIProviderPrefixAndDotVersion(t *testing.T) {
	p, ok := Lookup("openai/gpt-5.6-sol")
	if !ok {
		t.Fatal("expected openai/gpt-5.6-sol to be known")
	}
	if !almostEqual(p.Input, 4.0) || !almostEqual(p.Output, 20.0) {
		t.Fatalf("unexpected price: %+v", p)
	}
}

func TestLookupLongestPrefixMatch(t *testing.T) {
	// No exact entry for this id; it extends a known id with a trailing
	// qualifier that is not a date suffix, so prefix matching must kick in.
	p, ok := Lookup("claude-opus-4-5-preview")
	if !ok {
		t.Fatal("expected claude-opus-4-5-preview to resolve via prefix match")
	}
	want, ok2 := Lookup("claude-opus-4-5")
	if !ok2 {
		t.Fatal("claude-opus-4-5 should be a known base model")
	}
	if p != want {
		t.Fatalf("prefix match price mismatch: got %+v want %+v", p, want)
	}

	// Prefix matching must respect '-' boundaries: "claude-opus-4" should
	// not match a hypothetical "claude-opus-45..." id.
	if _, ok := Lookup("claude-opus-45-fake-model"); ok {
		t.Fatal("expected claude-opus-45-fake-model to be unknown (no boundary match)")
	}
}

func TestLookupUnknownModel(t *testing.T) {
	if _, ok := Lookup("totally-made-up-model-xyz"); ok {
		t.Fatal("expected unknown model to report ok=false")
	}
}

func TestLookupLocalModelIsZero(t *testing.T) {
	p, ok := Lookup("qwen3.8:27b")
	if !ok {
		t.Fatal("expected local model to be known")
	}
	if p != (Price{}) {
		t.Fatalf("expected zero price for local model, got %+v", p)
	}

	p2, ok2 := Lookup("my-ollama-model")
	if !ok2 {
		t.Fatal("expected ollama-named model to be known")
	}
	if p2 != (Price{}) {
		t.Fatalf("expected zero price for ollama model, got %+v", p2)
	}
}

func TestEstimateArithmetic(t *testing.T) {
	usd, known := Estimate("claude-sonnet-4-5", Tokens{
		Input:      1_000_000,
		Output:     500_000,
		CacheRead:  2_000_000,
		CacheWrite: 100_000,
	})
	if !known {
		t.Fatal("expected claude-sonnet-4-5 to be known")
	}
	// input: 1M * 3.0  = 3.0
	// output: 0.5M * 15.0 = 7.5
	// cache_read: 2M * 0.30 = 0.6
	// cache_write: 0.1M * 3.75 = 0.375
	want := 3.0 + 7.5 + 0.6 + 0.375
	if !almostEqual(usd, want) {
		t.Fatalf("Estimate() = %v, want %v", usd, want)
	}
}

func TestEstimateUnknownModel(t *testing.T) {
	usd, known := Estimate("totally-made-up-model-xyz", Tokens{Input: 1000})
	if known {
		t.Fatal("expected unknown model")
	}
	if usd != 0 {
		t.Fatalf("expected 0 usd for unknown model, got %v", usd)
	}
}

func TestEstimateLocalModelZero(t *testing.T) {
	usd, known := Estimate("llama3:70b", Tokens{Input: 1_000_000, Output: 1_000_000})
	if !known {
		t.Fatal("expected local model to be known")
	}
	if usd != 0 {
		t.Fatalf("expected 0 usd for local model, got %v", usd)
	}
}

func TestEstimateZeroTokens(t *testing.T) {
	usd, known := Estimate("claude-haiku-4-5", Tokens{})
	if !known {
		t.Fatal("expected claude-haiku-4-5 to be known")
	}
	if usd != 0 {
		t.Fatalf("expected 0 usd for zero tokens, got %v", usd)
	}
}
