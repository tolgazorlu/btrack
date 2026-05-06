package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tolgazorlu/btrack/internal/config"
	"github.com/tolgazorlu/btrack/internal/db"
	gh "github.com/tolgazorlu/btrack/internal/github"
	"github.com/tolgazorlu/btrack/internal/ui"
)

var githubCmd = &cobra.Command{
	Use:   "github",
	Short: "GitHub integration — connect, sync, and view contributions",
	Long: `Link your GitHub account to enrich btrack with real commit and PR data.

Commands:
  btrack github connect   Authenticate with a Personal Access Token
  btrack github status    Show connected account and today's activity
  btrack github sync      Import today's commits as btrack sessions

Once connected:
  · btrack ai sum    includes your real commits and PRs in the standup
  · btrack ai ins    includes GitHub contribution stats in insights
  · btrack h         shows GitHub activity below your sessions
  · btrack h -w      shows per-day commit/PR counts

How to get a token:
  github.com/settings/tokens/new
  Required scopes: read:user, repo`,
}

// ─── connect ─────────────────────────────────────────────────────────────────

var githubConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Authenticate with a GitHub Personal Access Token",
	Long: `Link btrack to your GitHub account with a Personal Access Token.

Option A — Classic token (simpler):
  1. Go to: github.com/settings/tokens/new
  2. Token name: btrack
  3. Scopes: check "read:user" and "repo"
  4. Click Generate token, copy it, paste below

Option B — Fine-grained token (more secure):
  1. Go to: github.com/settings/tokens?type=beta
  2. Repository access: All repositories (or select specific ones)
  3. Permissions: Contents = Read-only, Metadata = Read-only
  4. Click Generate token, copy it, paste below

The token is stored in: ~/.config/btrack/config.yaml
It is never sent anywhere except the GitHub API.

After connecting, these commands are enriched:
  btrack ai sum       standup includes your real commits and PRs
  btrack ai ins       insights include GitHub contribution stats
  btrack h            shows GitHub activity below sessions
  btrack h -w         shows per-day commit/PR count`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ui.Header("github connect", "")
		ui.Hint("github.com/settings/tokens/new  →  scopes: read:user, repo")
		ui.Blank()
		fmt.Printf("%s%s ", ui.Indent, ui.StyleDimmed.Render("token:"))

		reader := bufio.NewReader(os.Stdin)
		pat, _ := reader.ReadString('\n')
		pat = strings.TrimSpace(pat)
		if pat == "" {
			return fmt.Errorf("no token provided")
		}

		ui.Blank()
		ui.Dim("verifying token…")

		client := gh.NewClient(pat, "")
		user, err := client.GetUser()
		if err != nil {
			return fmt.Errorf("token validation failed: %w", err)
		}

		if err := config.SaveGitHub(pat, user.Login); err != nil {
			return fmt.Errorf("save config: %w", err)
		}

		displayName := user.Login
		if user.Name != "" {
			displayName = user.Name + " (@" + user.Login + ")"
		}

		ui.Blank()
		ui.OK("connected as " + ui.StyleHighlight.Render(displayName))
		ui.Hint("`btrack github status` to see your activity")
		ui.Blank()
		return nil
	},
}

// ─── status ──────────────────────────────────────────────────────────────────

var githubStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show connected GitHub account and today's activity",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.GitHub.PAT == "" {
			ui.Blank()
			ui.Warn("not connected — run `btrack github connect`")
			ui.Blank()
			return nil
		}

		client := gh.NewClient(cfg.GitHub.PAT, cfg.GitHub.Username)

		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).UTC()
		end := start.Add(24 * time.Hour)

		ui.Header("github", "@"+cfg.GitHub.Username)
		ui.Dim("fetching today's activity…")
		ui.Blank()

		activity, err := client.GetActivity(start, end)
		if err != nil {
			return fmt.Errorf("github: %w", err)
		}

		printGitHubActivity(activity, "today's activity")
		return nil
	},
}

// ─── sync ────────────────────────────────────────────────────────────────────

var githubSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Import today's GitHub commits as btrack sessions",
	Long: `Pull today's GitHub commits and create btrack sessions from them.

Each repository's commits are grouped into a single session.
Duration = time from first commit to last commit + 15 minutes.

Auto-tagging from commit message keywords:
  fix / bug      ->  #bugfix
  feat / add     ->  #feature
  refactor       ->  #refactor
  test           ->  #test
  doc / readme   ->  #docs
  Explicit #tags in commit messages are carried over as-is.

Note: each sync call creates new sessions — running it twice will create
duplicate sessions. Use it once per day or after a long GitHub session.

Requires: btrack github connect`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if cfg.GitHub.PAT == "" {
			return fmt.Errorf("not connected — run `btrack github connect` first")
		}

		store, err := db.Open(cfg)
		if err != nil {
			return err
		}
		defer store.Close()

		client := gh.NewClient(cfg.GitHub.PAT, cfg.GitHub.Username)

		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).UTC()
		end := start.Add(24 * time.Hour)

		ui.Header("github sync", "")
		ui.Dim("syncing today's activity…")
		ui.Blank()

		activity, err := client.GetActivity(start, end)
		if err != nil {
			return fmt.Errorf("github: %w", err)
		}

		if activity.IsEmpty() {
			ui.Hint("no GitHub activity found for today")
			ui.Blank()
			return nil
		}

		printGitHubActivity(activity, "imported")

		commits, prs := syncToSessions(store, activity)
		total := commits + prs

		if total == 0 {
			ui.Hint("nothing new to import — no commits or PRs for today")
			ui.Blank()
			return nil
		}

		var parts []string
		if commits > 0 {
			parts = append(parts, fmt.Sprintf("%d commit", commits))
		}
		if prs > 0 {
			parts = append(parts, fmt.Sprintf("%d PR", prs))
		}
		ui.OK("imported " + ui.StyleHighlight.Render(strings.Join(parts, " · ")))
		ui.Blank()
		return nil
	},
}

// syncToSessions imports commits and PRs as btrack sessions.
// Returns (commitSessions, prSessions) created counts.
func syncToSessions(store db.Store, activity *gh.Activity) (int, int) {
	commitCount := 0

	// Group commits by repo → one session per repo.
	repoCommits := map[string][]*gh.Commit{}
	for _, c := range activity.Commits {
		repoCommits[c.Repo] = append(repoCommits[c.Repo], c)
	}

	for repo, commits := range repoCommits {
		if len(commits) == 0 {
			continue
		}

		earliest, latest := commits[0].Time, commits[0].Time
		var msgs []string
		for _, c := range commits {
			if c.Time.Before(earliest) {
				earliest = c.Time
			}
			if c.Time.After(latest) {
				latest = c.Time
			}
			msgs = append(msgs, c.Message)
		}

		endTime := latest.Add(15 * time.Minute)

		repoName := repo
		if parts := strings.SplitN(repo, "/", 2); len(parts) == 2 {
			repoName = parts[1]
		}

		message := strings.Join(msgs, " | ")
		if len(message) > 200 {
			message = message[:197] + "..."
		}

		sess := &db.Session{
			TaskName:  "GitHub: " + repoName,
			StartTime: earliest.Local(),
			EndTime:   &endTime,
			Message:   message,
			Tags:      tagsFromMessages(msgs),
			GitRepo:   repo,
		}
		if store.CreateSession(sess) == nil {
			commitCount++
		}
	}

	// Create a session for each merged or opened PR.
	prCount := 0
	for _, pr := range activity.PullRequests {
		if pr.Action != "merged" && pr.Action != "opened" {
			continue
		}

		repoName := pr.Repo
		if parts := strings.SplitN(pr.Repo, "/", 2); len(parts) == 2 {
			repoName = parts[1]
		}

		taskName := fmt.Sprintf("PR #%d: %s", pr.Number, pr.Title)
		if len(taskName) > 60 {
			taskName = taskName[:57] + "..."
		}

		endTime := pr.Time.Local().Add(30 * time.Minute)
		startTime := pr.Time.Local()

		tags := []string{"#pr"}
		if pr.Action == "merged" {
			tags = append(tags, "#merged")
		}

		sess := &db.Session{
			TaskName:  taskName,
			StartTime: startTime,
			EndTime:   &endTime,
			Message:   fmt.Sprintf("[%s] %s — %s", pr.Action, pr.Title, repoName),
			Tags:      tags,
			GitRepo:   pr.Repo,
		}
		if store.CreateSession(sess) == nil {
			prCount++
		}
	}

	return commitCount, prCount
}

// tagsFromMessages extracts btrack tags from commit message keywords.
func tagsFromMessages(msgs []string) []string {
	set := map[string]bool{}
	for _, msg := range msgs {
		lower := strings.ToLower(msg)
		if strings.Contains(lower, "fix") || strings.Contains(lower, "bug") {
			set["#bugfix"] = true
		}
		if strings.Contains(lower, "feat") || strings.Contains(lower, "add") || strings.Contains(lower, "new") {
			set["#feature"] = true
		}
		if strings.Contains(lower, "refactor") || strings.Contains(lower, "clean") {
			set["#refactor"] = true
		}
		if strings.Contains(lower, "test") {
			set["#test"] = true
		}
		if strings.Contains(lower, "doc") || strings.Contains(lower, "readme") {
			set["#docs"] = true
		}
		// Carry explicit #tags from commit messages
		for _, word := range strings.Fields(msg) {
			if strings.HasPrefix(word, "#") && len(word) > 1 {
				set[strings.ToLower(strings.TrimRight(word, ".,;:"))] = true
			}
		}
	}
	tags := make([]string, 0, len(set))
	for t := range set {
		tags = append(tags, t)
	}
	return tags
}

// ─── shared rendering ────────────────────────────────────────────────────────

// printGitHubActivity renders a GitHub activity block to stdout.
func printGitHubActivity(activity *gh.Activity, title string) {
	if activity.IsEmpty() {
		ui.Hint("no GitHub activity found")
		ui.Blank()
		return
	}

	if title != "" {
		ui.Section(title)
	}

	if len(activity.Commits) > 0 {
		ui.Section(fmt.Sprintf("commits · %d", len(activity.Commits)))
		for _, c := range activity.Commits {
			msg := c.Message
			if len(msg) > 50 {
				msg = msg[:47] + "..."
			}
			repo := c.Repo
			if idx := strings.Index(repo, "/"); idx != -1 {
				repo = repo[idx+1:]
			}
			fmt.Printf("%s%s  %s  %s\n",
				ui.Indent,
				ui.StyleDimmed.Render(c.SHA),
				ui.StyleHighlight.Render(msg),
				ui.StyleDimmed.Render(repo),
			)
		}
		ui.Blank()
	}

	if len(activity.PullRequests) > 0 {
		ui.Section(fmt.Sprintf("pull requests · %d", len(activity.PullRequests)))
		for _, pr := range activity.PullRequests {
			t := pr.Title
			if len(t) > 50 {
				t = t[:47] + "..."
			}
			var label string
			switch pr.Action {
			case "merged":
				label = ui.StyleSuccess.Render("merged  ")
			case "opened":
				label = ui.StyleHighlight.Render("opened  ")
			case "reviewed":
				label = ui.StyleWarning.Render("reviewed")
			default:
				label = ui.StyleDimmed.Render(pr.Action)
			}
			fmt.Printf("%s%s  %s\n", ui.Indent, label, ui.StyleHighlight.Render(t))
		}
		ui.Blank()
	}

	if len(activity.Issues) > 0 {
		ui.Section(fmt.Sprintf("issues · %d", len(activity.Issues)))
		for _, iss := range activity.Issues {
			t := iss.Title
			if len(t) > 54 {
				t = t[:51] + "..."
			}
			fmt.Printf("%s%-8s  %s\n",
				ui.Indent,
				ui.StyleDimmed.Render(iss.Action),
				ui.StyleHighlight.Render(t),
			)
		}
		ui.Blank()
	}
}

// ghClientFromConfig returns a GitHub client if connected, or nil.
func ghClientFromConfig(cfg *config.Config) *gh.Client {
	if cfg.GitHub.PAT == "" || cfg.GitHub.Username == "" {
		return nil
	}
	return gh.NewClient(cfg.GitHub.PAT, cfg.GitHub.Username)
}

func init() {
	githubCmd.AddCommand(githubConnectCmd, githubStatusCmd, githubSyncCmd)
	rootCmd.AddCommand(githubCmd)
}
