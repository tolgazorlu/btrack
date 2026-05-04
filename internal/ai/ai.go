package ai

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
)

// Provider is the AI backend interface.
type Provider interface {
	Complete(ctx context.Context, prompt string) (string, error)
	Name() string
}

// NewProvider creates the configured AI provider.
func NewProvider(cfg *config.Config) (Provider, error) {
	key := cfg.AI.ActiveKey()
	if key == "" {
		return nil, fmt.Errorf(
			"no AI key configured — run `btrack ai setup` to add one",
		)
	}
	switch cfg.AI.Provider {
	case "claude":
		return NewClaudeProvider(key, cfg.AI.Model), nil
	case "gemini":
		return NewGeminiProvider(key, cfg.AI.Model), nil
	default:
		return NewOpenAIProvider(key, cfg.AI.Model), nil
	}
}

// NewProviderFor creates a provider for a specific provider name + key (used by setup wizard).
func NewProviderFor(provider, key string) (Provider, error) {
	switch provider {
	case "claude":
		return NewClaudeProvider(key, ""), nil
	case "gemini":
		return NewGeminiProvider(key, ""), nil
	case "openai":
		return NewOpenAIProvider(key, ""), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// SummarizeStandup generates a daily standup from recent sessions.
func SummarizeStandup(ctx context.Context, p Provider, sessions []*db.Session, logs map[int64][]*db.LogEntry) (string, error) {
	gitDiff, _ := getGitContext()
	prompt := buildStandupPrompt(sessions, logs, gitDiff, "")
	return p.Complete(ctx, prompt)
}

// SummarizeStandupWithGitHub generates a standup enriched with GitHub activity.
func SummarizeStandupWithGitHub(ctx context.Context, p Provider, sessions []*db.Session, logs map[int64][]*db.LogEntry, githubActivity string) (string, error) {
	gitDiff, _ := getGitContext()
	prompt := buildStandupPrompt(sessions, logs, gitDiff, githubActivity)
	return p.Complete(ctx, prompt)
}

// AnalyzeStatsWithGitHub asks the AI to interpret stats enriched with GitHub data.
func AnalyzeStatsWithGitHub(ctx context.Context, p Provider, statsJSON, githubActivity string) (string, error) {
	prompt := fmt.Sprintf(`Analyze these developer time-tracking statistics and GitHub activity, then provide actionable insights.
Be specific, encouraging, and concise (under 200 words).

Focus on:
- Productivity patterns (best times, session lengths)
- Tag/category distribution
- GitHub contribution patterns (commits, PRs, reviews)
- Correlation between tracked hours and GitHub output
- One positive observation and one actionable suggestion

Stats:
%s

GitHub Activity:
%s`, statsJSON, githubActivity)
	return p.Complete(ctx, prompt)
}

// SuggestCommitMessage proposes a stop message from git diff + log notes.
func SuggestCommitMessage(ctx context.Context, p Provider, taskName string, notes []string) (string, error) {
	gitDiff, _ := getGitContext()
	prompt := buildCommitPrompt(taskName, notes, gitDiff)
	return p.Complete(ctx, prompt)
}

// AnalyzeStats asks the AI to interpret tracking statistics.
func AnalyzeStats(ctx context.Context, p Provider, statsJSON string) (string, error) {
	prompt := fmt.Sprintf(`Analyze these developer time-tracking statistics and provide actionable insights.
Be specific, encouraging, and concise (under 200 words).

Focus on:
- Productivity patterns (best times, session lengths)
- Tag/category distribution (too much of one thing?)
- Suggestions for improvement
- One positive observation

Stats:
%s`, statsJSON)
	return p.Complete(ctx, prompt)
}

// CategorizeTask detects common tags from a message.
func CategorizeTask(msg string) []string {
	lower := strings.ToLower(msg)
	tagMap := map[string][]string{
		"#feature":  {"feat", "feature", "add", "new", "implement"},
		"#bugfix":   {"fix", "bug", "patch", "resolve", "issue", "error"},
		"#refactor": {"refactor", "clean", "cleanup", "restructure"},
		"#test":     {"test", "spec", "coverage", "unit", "integration"},
		"#docs":     {"doc", "readme", "comment", "changelog"},
		"#ci":       {"ci", "cd", "pipeline", "workflow", "deploy"},
	}
	existing := map[string]bool{}
	for _, w := range strings.Fields(msg) {
		if strings.HasPrefix(w, "#") {
			existing[strings.ToLower(w)] = true
		}
	}
	var tags []string
	for tag, keywords := range tagMap {
		if existing[tag] {
			continue
		}
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				tags = append(tags, tag)
				break
			}
		}
	}
	return tags
}

// ValidateKey makes a minimal API call to confirm the key works.
func ValidateKey(ctx context.Context, p Provider) error {
	_, err := p.Complete(ctx, "Reply with only the word: ok")
	return err
}

func buildStandupPrompt(sessions []*db.Session, logs map[int64][]*db.LogEntry, gitDiff, githubActivity string) string {
	var sb strings.Builder
	sb.WriteString("Generate a concise professional daily standup from these time tracking entries:\n\n")
	for _, s := range sessions {
		sb.WriteString(fmt.Sprintf("Task: %s (%.1f min)\n", s.TaskName, s.Duration().Minutes()))
		if s.Message != "" {
			sb.WriteString(fmt.Sprintf("  Summary: %s\n", s.Message))
		}
		if entries, ok := logs[s.ID]; ok {
			for _, e := range entries {
				sb.WriteString(fmt.Sprintf("  - %s\n", e.Note))
			}
		}
	}
	if gitDiff != "" {
		sb.WriteString("\nLocal git changes:\n")
		sb.WriteString(gitDiff)
	}
	if githubActivity != "" {
		sb.WriteString("\nGitHub contributions:\n")
		sb.WriteString(githubActivity)
	}
	sb.WriteString("\nFormat: What I did (bullets), blockers (if any), what's next. Keep under 150 words.")
	return sb.String()
}

func buildCommitPrompt(taskName string, notes []string, gitDiff string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Suggest a concise commit-style message for this time session:\n\nTask: %s\n", taskName))
	if len(notes) > 0 {
		sb.WriteString("Notes:\n")
		for _, n := range notes {
			sb.WriteString(fmt.Sprintf("  - %s\n", n))
		}
	}
	if gitDiff != "" {
		sb.WriteString("\nGit diff summary:\n")
		sb.WriteString(gitDiff)
	}
	sb.WriteString("\nRespond with ONLY the message (max 72 chars, imperative mood).")
	return sb.String()
}

func getGitContext() (string, error) {
	stat, err := exec.Command("git", "diff", "--stat", "HEAD~1", "HEAD").Output()
	if err != nil {
		stat, err = exec.Command("git", "diff", "--stat").Output()
	}
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(string(stat))
	if len(out) > 1500 {
		out = out[:1500] + "\n... (truncated)"
	}
	return out, nil
}
