package cmd

import "testing"

func TestSlugCannotEscapeDirectory(t *testing.T) {
	cases := map[string]string{
		"Yalçınkaya B2B":      "yalcinkaya-b2b",
		"../../.ssh/x":        "sshx",
		`C:\Windows\System32`: "cwindowssystem32",
		"a & calc.exe":        "a--calcexe",
		"///":                 "client",
		"":                    "client",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}
