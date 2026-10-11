package cmd

import "testing"

func TestCSVSafeNeutralisesFormulas(t *testing.T) {
	cases := map[string]string{
		"":                         "",
		"fix login bug":            "fix login bug",
		"=HYPERLINK(\"http://x\")": "'=HYPERLINK(\"http://x\")",
		"+1+1":                     "'+1+1",
		"-2+3":                     "'-2+3",
		"@SUM(A1:A2)":              "'@SUM(A1:A2)",
		"\t=1":                     "'\t=1",
		"\r=1":                     "'\r=1",
		"feature/agent-ledger":     "feature/agent-ledger",
	}
	for in, want := range cases {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}
