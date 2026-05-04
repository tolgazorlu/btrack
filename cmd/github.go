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
  · btrack day       shows GitHub activity below your sessions
  · btrack week      shows per-day commit/PR counts

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
  btrack day          shows GitHub activity below sessions
  btrack week         shows per-day commit/PR count`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println()
		fmt.Printf("  %s\n", ui.StyleTitle.Render("GitHub Connect"))
		fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("github.com/settings/tokens/new  →  scopes: read:user, repo"))
		fmt.Printf("  %s ", ui.StyleHighlight.Render("Personal Access Token:"))

		reader := bufio.NewReader(os.Stdin)
		pat, _ := reader.ReadString('\n')
		pat = strings.TrimSpace(pat)
		if pat == "" {
			return fmt.Errorf("no token provided")
		}

		fmt.Print(ui.StyleDimmed.Render("\n  verifying token...\n"))

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

		fmt.Printf("\n  %s  Connected as %s\n\n",
			ui.StyleSuccess.Render("✓"),
			ui.StyleHighlight.Render(displayName),
		)
		fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("run `btrack github status` to see your activity"))
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
			fmt.Printf("\n  %s\n\n", ui.StyleWarning.Render("not connected — run `btrack github connect`"))
			return nil
		}

		client := gh.NewClient(cfg.GitHub.PAT, cfg.GitHub.Username)

		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).UTC()
		end := start.Add(24 * time.Hour)

		fmt.Printf("\n  %s  @%s\n\n",
			ui.StyleSuccess.Render("✓"),
			ui.StyleHighlight.Render(cfg.GitHub.Username),
		)
		fmt.Print(ui.StyleDimmed.Render("  fetching today's activity...\n\n"))

		activity, err := client.GetActivity(start, end)
		if err != nil {
			return fmt.Errorf("github: %w", err)
		}

		printGitHubActivity(activity, "Today's GitHub Activity")
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

		fmt.Print(ui.StyleDimmed.Render("\n  syncing GitHub activity...\n\n"))

		activity, err := client.GetActivity(start, end)
		if err != nil {
			return fmt.Errorf("github: %w", err)
		}

		if activity.IsEmpty() {
			fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("no GitHub activity found for today"))
			return nil
		}

		printGitHubActivity(activity, "Imported activity")

		commits, prs := syncToSessions(store, activity)
		total := commits + prs

		if total == 0 {
			fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("nothing new to import — no commits or PRs found for today"))
			return nil
		}

		var parts []string
		if commits > 0 {
			parts = append(parts, fmt.Sprintf("%d commit session(s)", commits))
		}
		if prs > 0 {
			parts = append(parts, fmt.Sprintf("%d PR session(s)", prs))
		}
		fmt.Printf("  %s  Imported: %s\n\n",
			ui.StyleSuccess.Render("✓"),
			strings.Join(parts, "  ·  "),
		)
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
		fmt.Printf("  %s\n\n", ui.StyleDimmed.Render("no GitHub activity found"))
		return
	}

	sep := ui.StyleDimmed.Render(strings.Repeat("─", 54))
	fmt.Printf("  %s\n", ui.StyleHighlight.Render(title))
	fmt.Println("  " + sep)

	if len(activity.Commits) > 0 {
		fmt.Printf("\n  %s\n", ui.StyleDimmed.Render(fmt.Sprintf("commits  (%d)", len(activity.Commits))))
		for _, c := range activity.Commits {
			msg := c.Message
			if len(msg) > 54 {
				msg = msg[:51] + "..."
			}
			repo := c.Repo
			if idx := strings.Index(repo, "/"); idx != -1 {
				repo = repo[idx+1:]
			}
			fmt.Printf("  %s  %s  %s\n",
				ui.StyleSubtle.Render(c.SHA),
				ui.StyleDimmed.Render(msg),
				ui.StyleSubtle.Render(repo),
			)
		}
	}

	if len(activity.PullRequests) > 0 {
		fmt.Printf("\n  %s\n", ui.StyleDimmed.Render(fmt.Sprintf("pull requests  (%d)", len(activity.PullRequests))))
		for _, pr := range activity.PullRequests {
			title := pr.Title
			if len(title) > 50 {
				title = title[:47] + "..."
			}
			var actionStr string
			switch pr.Action {
			case "merged":
				actionStr = ui.StyleSuccess.Render("[merged]")
			case "opened":
				actionStr = ui.StyleHighlight.Render("[opened]")
			case "reviewed":
				actionStr = ui.StyleWarning.Render("[reviewed]")
			default:
				actionStr = ui.StyleDimmed.Render("[" + pr.Action + "]")
			}
			fmt.Printf("  %s  %s\n", actionStr, ui.StyleDimmed.Render(title))
		}
	}

	if len(activity.Issues) > 0 {
		fmt.Printf("\n  %s\n", ui.StyleDimmed.Render(fmt.Sprintf("issues  (%d)", len(activity.Issues))))
		for _, iss := range activity.Issues {
			title := iss.Title
			if len(title) > 54 {
				title = title[:51] + "..."
			}
			fmt.Printf("  %s  %s\n",
				ui.StyleDimmed.Render(fmt.Sprintf("[%s]", iss.Action)),
				ui.StyleDimmed.Render(title),
			)
		}
	}
	fmt.Println()
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
