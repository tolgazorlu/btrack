package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseProjectFile_FullExample(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".btrack")
	contents := `# header comment
project       = myapp
task_prefix   = [myapp]
description   = Customer-facing web app
daily_hours   = 6
billing_rate  = 150.00
default_tags  = #frontend, #react, ops
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}

	pf, err := parseProjectFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pf.Project != "myapp" {
		t.Errorf("project = %q", pf.Project)
	}
	if pf.TaskPrefix != "[myapp]" {
		t.Errorf("task_prefix = %q", pf.TaskPrefix)
	}
	if pf.Description != "Customer-facing web app" {
		t.Errorf("description = %q", pf.Description)
	}
	if pf.DailyHours != 6 {
		t.Errorf("daily_hours = %d", pf.DailyHours)
	}
	if pf.BillingRate != 150.00 {
		t.Errorf("billing_rate = %v", pf.BillingRate)
	}
	want := []string{"#frontend", "#react", "#ops"}
	if !reflect.DeepEqual(pf.DefaultTags, want) {
		t.Errorf("default_tags = %#v, want %#v", pf.DefaultTags, want)
	}
}

func TestParseProjectFile_TagsAreNotMistakenForComments(t *testing.T) {
	// Regression: an earlier version stripped " #" mid-value as an inline
	// comment, which silently truncated tag lists like "#a, #b" → "#a".
	dir := t.TempDir()
	path := filepath.Join(dir, ".btrack")
	if err := os.WriteFile(path, []byte("default_tags = #a, #b, #c\n"), 0600); err != nil {
		t.Fatal(err)
	}
	pf, err := parseProjectFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(pf.DefaultTags); got != 3 {
		t.Fatalf("got %d tags, want 3 (%v)", got, pf.DefaultTags)
	}
}

func TestParseTagList(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"frontend", []string{"#frontend"}},
		{"#frontend", []string{"#frontend"}},
		{"frontend, react", []string{"#frontend", "#react"}},
		{"frontend,react,ops", []string{"#frontend", "#react", "#ops"}},
		{"#a #b #c", []string{"#a", "#b", "#c"}},
		{"FRONTEND, Frontend, frontend", []string{"#frontend"}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := ParseTagList(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParseTagList(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestRender_RoundTrip(t *testing.T) {
	in := &ProjectFile{
		Project:     "myapp",
		TaskPrefix:  "[myapp]",
		Description: "demo",
		DailyHours:  6,
		BillingRate: 150,
		DefaultTags: []string{"#frontend", "#react"},
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".btrack")
	if err := os.WriteFile(path, []byte(in.Render()), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := parseProjectFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\nin  = %#v\nout = %#v", in, out)
	}
}
