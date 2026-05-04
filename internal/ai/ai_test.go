package ai

import (
	"testing"
)

func TestCategorizeTask(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		wantTags []string
	}{
		{
			name:     "fix keyword triggers bugfix",
			msg:      "fix login redirect issue",
			wantTags: []string{"#bugfix"},
		},
		{
			name:     "feat keyword triggers feature",
			msg:      "add new dashboard feature",
			wantTags: []string{"#feature"},
		},
		{
			name:     "refactor keyword",
			msg:      "refactor auth middleware",
			wantTags: []string{"#refactor"},
		},
		{
			name:     "test keyword",
			msg:      "write unit tests for auth",
			wantTags: []string{"#test"},
		},
		{
			name:     "doc keyword",
			msg:      "update readme and docs",
			wantTags: []string{"#docs"},
		},
		{
			name:     "explicit tag not duplicated",
			msg:      "fix login bug #bugfix",
			wantTags: []string{},
		},
		{
			name:     "ci keyword",
			msg:      "fix ci pipeline workflow",
			wantTags: []string{"#ci"},
		},
		{
			name:     "no keywords returns empty",
			msg:      "morning standup meeting",
			wantTags: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CategorizeTask(tt.msg)
			if len(tt.wantTags) == 0 {
				if len(got) != 0 {
					t.Errorf("CategorizeTask(%q) = %v, want empty", tt.msg, got)
				}
				return
			}
			gotSet := map[string]bool{}
			for _, g := range got {
				gotSet[g] = true
			}
			for _, want := range tt.wantTags {
				if !gotSet[want] {
					t.Errorf("CategorizeTask(%q) missing tag %q, got %v", tt.msg, want, got)
				}
			}
		})
	}
}
