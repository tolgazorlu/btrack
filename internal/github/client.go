package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://api.github.com"

type Client struct {
	pat      string
	username string
	http     *http.Client
}

func NewClient(pat, username string) *Client {
	return &Client{
		pat:      pat,
		username: username,
		http:     &http.Client{Timeout: 15 * time.Second},
	}
}

type UserInfo struct {
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Commit struct {
	SHA     string
	Message string
	Repo    string
	Time    time.Time
	URL     string
}

type PullRequest struct {
	Number int
	Title  string
	Repo   string
	Action string // opened | merged | closed | reviewed
	Time   time.Time
	URL    string
}

type Issue struct {
	Number int
	Title  string
	Repo   string
	Action string
	Time   time.Time
	URL    string
}

type Activity struct {
	Date         time.Time
	Commits      []*Commit
	PullRequests []*PullRequest
	Issues       []*Issue
}

func (a *Activity) IsEmpty() bool {
	return len(a.Commits) == 0 && len(a.PullRequests) == 0 && len(a.Issues) == 0
}

// Summary returns a plain-text description of the activity for AI prompts.
func (a *Activity) Summary() string {
	if a.IsEmpty() {
		return ""
	}
	var sb strings.Builder
	if len(a.Commits) > 0 {
		sb.WriteString(fmt.Sprintf("Commits (%d):\n", len(a.Commits)))
		for _, c := range a.Commits {
			sb.WriteString(fmt.Sprintf("  [%s] %s — %s\n", c.SHA, c.Message, c.Repo))
		}
	}
	if len(a.PullRequests) > 0 {
		sb.WriteString(fmt.Sprintf("Pull Requests (%d):\n", len(a.PullRequests)))
		for _, pr := range a.PullRequests {
			sb.WriteString(fmt.Sprintf("  #%d %s [%s] — %s\n", pr.Number, pr.Title, pr.Action, pr.Repo))
		}
	}
	if len(a.Issues) > 0 {
		sb.WriteString(fmt.Sprintf("Issues (%d):\n", len(a.Issues)))
		for _, iss := range a.Issues {
			sb.WriteString(fmt.Sprintf("  #%d %s [%s] — %s\n", iss.Number, iss.Title, iss.Action, iss.Repo))
		}
	}
	return sb.String()
}

func (c *Client) do(path string, accept string, v interface{}) error {
	req, err := http.NewRequest("GET", baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.pat)
	if accept != "" {
		req.Header.Set("Accept", accept)
	} else {
		req.Header.Set("Accept", "application/vnd.github+json")
	}
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("github request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("invalid token — run `btrack github connect` to reconnect")
	}
	if resp.StatusCode == 403 {
		return fmt.Errorf("rate limited or insufficient scopes")
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *Client) GetUser() (*UserInfo, error) {
	var u UserInfo
	if err := c.do("/user", "", &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// searchCommits uses the GitHub search API to find commits authored by the user
// in a date range. More reliable than events API for private repos.
func (c *Client) searchCommits(since, until time.Time) ([]*Commit, error) {
	dateFilter := fmt.Sprintf("%s..%s",
		since.Local().Format("2006-01-02"),
		until.Local().Format("2006-01-02"),
	)
	path := fmt.Sprintf(
		"/search/commits?q=author%%3A%s+committer-date%%3A%s&per_page=100&sort=committer-date&order=desc",
		c.username, dateFilter,
	)

	type searchResult struct {
		Items []struct {
			SHA    string `json:"sha"`
			HTMLURL string `json:"html_url"`
			Commit struct {
				Message   string `json:"message"`
				Committer struct {
					Date time.Time `json:"date"`
				} `json:"committer"`
			} `json:"commit"`
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		} `json:"items"`
	}

	var result searchResult
	// Search commits API requires the cloak preview header.
	if err := c.do(path, "application/vnd.github.cloak-preview+json", &result); err != nil {
		return nil, err
	}

	var commits []*Commit
	for _, item := range result.Items {
		sha := item.SHA
		if len(sha) > 7 {
			sha = sha[:7]
		}
		msg := strings.SplitN(item.Commit.Message, "\n", 2)[0]
		commits = append(commits, &Commit{
			SHA:     sha,
			Message: msg,
			Repo:    item.Repository.FullName,
			Time:    item.Commit.Committer.Date,
			URL:     item.HTMLURL,
		})
	}
	return commits, nil
}

// raw event types used only for decoding
type ghEvent struct {
	Type      string          `json:"type"`
	CreatedAt time.Time       `json:"created_at"`
	Repo      struct{ Name string `json:"name"` } `json:"repo"`
	Payload   json.RawMessage `json:"payload"`
}

type prPayload struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
		Merged  bool   `json:"merged"`
	} `json:"pull_request"`
}

type reviewPayload struct {
	Action string `json:"action"`
	Review struct{ HTMLURL string `json:"html_url"` } `json:"review"`
	PullRequest struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
	} `json:"pull_request"`
}

type issuePayload struct {
	Action string `json:"action"`
	Issue  struct {
		Number  int    `json:"number"`
		Title   string `json:"title"`
		HTMLURL string `json:"html_url"`
	} `json:"issue"`
}

// GetActivity fetches commits (via search API) and PR/issue events (via events API)
// between since and until.
func (c *Client) GetActivity(since, until time.Time) (*Activity, error) {
	act := &Activity{Date: since}

	// Use search API for commits — more reliable than events API for private repos.
	if commits, err := c.searchCommits(since, until); err == nil {
		act.Commits = commits
	}
	// Fall back to events API for PR and issue activity.
	for page := 1; page <= 5; page++ {
		var events []ghEvent
		path := fmt.Sprintf("/users/%s/events?per_page=100&page=%d", c.username, page)
		if err := c.do(path, "", &events); err != nil {
			return nil, err
		}
		if len(events) == 0 {
			break
		}

		done := false
		for _, e := range events {
			if e.CreatedAt.Before(since) {
				done = true
				break
			}
			if e.CreatedAt.After(until) {
				continue
			}

			switch e.Type {
			case "PullRequestEvent":
				var p prPayload
				if json.Unmarshal(e.Payload, &p) != nil {
					continue
				}
				action := p.Action
				if action == "closed" && p.PullRequest.Merged {
					action = "merged"
				}
				if action == "opened" || action == "merged" {
					act.PullRequests = append(act.PullRequests, &PullRequest{
						Number: p.PullRequest.Number,
						Title:  p.PullRequest.Title,
						Repo:   e.Repo.Name,
						Action: action,
						Time:   e.CreatedAt,
						URL:    p.PullRequest.HTMLURL,
					})
				}

			case "PullRequestReviewEvent":
				var p reviewPayload
				if json.Unmarshal(e.Payload, &p) != nil || p.Action != "submitted" {
					continue
				}
				act.PullRequests = append(act.PullRequests, &PullRequest{
					Number: p.PullRequest.Number,
					Title:  p.PullRequest.Title,
					Repo:   e.Repo.Name,
					Action: "reviewed",
					Time:   e.CreatedAt,
					URL:    p.Review.HTMLURL,
				})

			case "IssuesEvent":
				var p issuePayload
				if json.Unmarshal(e.Payload, &p) != nil {
					continue
				}
				if p.Action == "opened" || p.Action == "closed" {
					act.Issues = append(act.Issues, &Issue{
						Number: p.Issue.Number,
						Title:  p.Issue.Title,
						Repo:   e.Repo.Name,
						Action: p.Action,
						Time:   e.CreatedAt,
						URL:    p.Issue.HTMLURL,
					})
				}
			}
		}
		if done {
			break
		}
	}
	return act, nil
}

func (c *Client) Username() string { return c.username }

// SplitByDay partitions an Activity into per-day buckets keyed by "2006-01-02" (local time).
func SplitByDay(act *Activity) map[string]*Activity {
	m := map[string]*Activity{}
	add := func(key string) *Activity {
		if m[key] == nil {
			m[key] = &Activity{}
		}
		return m[key]
	}
	for _, c := range act.Commits {
		key := c.Time.Local().Format("2006-01-02")
		bucket := add(key)
		bucket.Commits = append(bucket.Commits, c)
	}
	for _, pr := range act.PullRequests {
		key := pr.Time.Local().Format("2006-01-02")
		bucket := add(key)
		bucket.PullRequests = append(bucket.PullRequests, pr)
	}
	for _, iss := range act.Issues {
		key := iss.Time.Local().Format("2006-01-02")
		bucket := add(key)
		bucket.Issues = append(bucket.Issues, iss)
	}
	return m
}
