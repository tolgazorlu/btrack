package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/tolgazorlu/btrack/internal/db"
	"github.com/tolgazorlu/btrack/internal/ingest"
	"github.com/tolgazorlu/btrack/internal/ledger"
)

// Instructions is sent to the client on initialize.
const Instructions = `btrack is the user's local ledger of work: hand-kept timesheet sessions
(what clients are billed from) plus AI agent sessions imported from
Claude Code, Codex and Hermes, linked to the commits they produced.

- To answer "what did I / we work on, for how long": call recap.
- After finishing a real piece of work, record it with log_work using the
  actual wall-clock start and end you observed. Never estimate or round
  times you did not see; if you do not know when work started, ask.
- Before preparing an invoice or timesheet, call unlogged_days to find
  days with agent work but nothing logged.
- Call sync first if imported data may be stale.
All times are the user's local time. Agent-measured times are heuristic.`

// periodProps are the shared period arguments.
var periodProps = map[string]any{
	"period":    map[string]any{"type": "string", "enum": []string{"day", "week", "month", "year"}, "description": "Calendar period containing today. Default week (Monday-based)."},
	"last_days": map[string]any{"type": "integer", "minimum": 1, "description": "The last N days including today. Overrides period."},
	"since":     map[string]any{"type": "string", "description": "Start date YYYY-MM-DD. Overrides period and last_days."},
	"until":     map[string]any{"type": "string", "description": "End date YYYY-MM-DD, inclusive."},
}

type periodArgs struct {
	Period   string `json:"period"`
	LastDays int    `json:"last_days"`
	Since    string `json:"since"`
	Until    string `json:"until"`
}

func (p periodArgs) resolve(now time.Time) (time.Time, time.Time, error) {
	from, to, _, err := ledger.PeriodSpec{Kind: p.Period, LastDays: p.LastDays, Since: p.Since, Until: p.Until}.Resolve(now)
	return from, to, err
}

func schema(required []string, props ...map[string]any) map[string]any {
	all := map[string]any{}
	for _, p := range props {
		for k, v := range p {
			all[k] = v
		}
	}
	s := map[string]any{"type": "object", "properties": all, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func decode(raw json.RawMessage, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

// Env is what the tools need from the host.
type Env struct {
	Open func() (db.Store, error) // opened per call, so the CLI and daemon are never locked out
	Home string
	Now  func() time.Time
}

func (e Env) with(fn func(db.Store) (any, error)) (any, error) {
	store, err := e.Open()
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return fn(store)
}

// Tools returns btrack's tools.
func Tools(e Env) []Tool {
	readOnly := map[string]any{"readOnlyHint": true, "openWorldHint": false}
	return []Tool{
		{
			Name:  "recap",
			Title: "Work recap",
			Description: "Summarise a period per project: the user's time (per day the larger of hand-logged and " +
				"measured keyboard time, never double counted), agent time, sessions, commits with their subjects, " +
				"and estimated AI cost.",
			InputSchema: schema(nil, periodProps, map[string]any{
				"project": map[string]any{"type": "string", "description": "Only this project."},
			}),
			Annotations: readOnly,
			Call: func(raw json.RawMessage) (any, error) {
				var a struct {
					periodArgs
					Project string `json:"project"`
				}
				if err := decode(raw, &a); err != nil {
					return nil, err
				}
				from, to, err := a.resolve(e.Now())
				if err != nil {
					return nil, err
				}
				return e.with(func(s db.Store) (any, error) { return ledger.LoadRecap(s, from, to, a.Project) })
			},
		},
		{
			Name:  "log_work",
			Title: "Log finished work",
			Description: "Record a finished piece of work on the user's timesheet (what clients are billed from). " +
				"Use the real start and end times you observed. Refuses future times, sessions over 16h and any " +
				"overlap with an existing timesheet entry.",
			InputSchema: schema([]string{"task", "start"}, map[string]any{
				"task":    map[string]any{"type": "string", "description": "What was done, as the commit message would say it."},
				"project": map[string]any{"type": "string", "description": "Project name as used on the timesheet."},
				"start":   map[string]any{"type": "string", "description": "Start: HH:MM (today), 'YYYY-MM-DD HH:MM' or RFC 3339."},
				"end":     map[string]any{"type": "string", "description": "End, same formats. Give end or minutes."},
				"minutes": map[string]any{"type": "integer", "minimum": 1, "description": "Duration in minutes, instead of end."},
				"message": map[string]any{"type": "string", "description": "Closing note, e.g. outcome and a #feature/#bugfix tag."},
			}),
			Annotations: map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": false, "openWorldHint": false},
			Call: func(raw json.RawMessage) (any, error) {
				var a struct {
					Task, Project, Start, End, Message string
					Minutes                            int
				}
				if err := decode(raw, &a); err != nil {
					return nil, err
				}
				now := e.Now()
				start, err := parseTime(a.Start, now)
				if err != nil {
					return nil, fmt.Errorf("start: %w", err)
				}
				var end time.Time
				switch {
				case a.End != "" && a.Minutes > 0:
					return nil, fmt.Errorf("give end or minutes, not both")
				case a.End != "":
					if end, err = parseTime(a.End, start); err != nil {
						return nil, fmt.Errorf("end: %w", err)
					}
				case a.Minutes > 0:
					end = start.Add(time.Duration(a.Minutes) * time.Minute)
				default:
					return nil, fmt.Errorf("give end or minutes")
				}
				return e.with(func(s db.Store) (any, error) {
					sess, err := ledger.Record(s, ledger.RecordInput{Task: a.Task, Project: a.Project, Message: a.Message, Start: start, End: end}, now)
					if err != nil {
						return nil, err
					}
					return map[string]any{"id": sess.ID, "task": sess.TaskName, "project": sess.Project,
						"start": sess.StartTime, "end": sess.EndTime, "minutes": int(end.Sub(start).Minutes())}, nil
				})
			},
		},
		{
			Name:        "list_sessions",
			Title:       "List sessions",
			Description: "List sessions in a period, newest first: timesheet entries and/or imported agent sessions.",
			InputSchema: schema(nil, periodProps, map[string]any{
				"project": map[string]any{"type": "string"},
				"source":  map[string]any{"type": "string", "description": "'manual' (timesheet), 'agents' (all imported), or one source: claude-code, codex, hermes. Default: all."},
				"limit":   map[string]any{"type": "integer", "minimum": 1, "maximum": 500, "description": "Default 50."},
			}),
			Annotations: readOnly,
			Call: func(raw json.RawMessage) (any, error) {
				var a struct {
					periodArgs
					Project, Source string
					Limit           int
				}
				if err := decode(raw, &a); err != nil {
					return nil, err
				}
				from, to, err := a.resolve(e.Now())
				if err != nil {
					return nil, err
				}
				if a.Limit <= 0 {
					a.Limit = 50
				}
				return e.with(func(s db.Store) (any, error) {
					all, err := s.GetSessionsInRange(from, to)
					if err != nil {
						return nil, err
					}
					var out []map[string]any
					for i := len(all) - 1; i >= 0 && len(out) < a.Limit; i-- {
						x := all[i]
						if a.Project != "" && x.Project != a.Project {
							continue
						}
						switch a.Source {
						case "":
						case "agents":
							if x.Source == db.SourceManual {
								continue
							}
						default:
							if x.Source != a.Source {
								continue
							}
						}
						out = append(out, sessionView(x))
					}
					return map[string]any{"from": from, "to": to, "sessions": out}, nil
				})
			},
		},
		{
			Name:  "unlogged_days",
			Title: "Days with unlogged agent work",
			Description: "Days where agent sessions show at least 15 minutes at the keyboard on a project but the " +
				"timesheet has no entry for it: hours probably worked and never logged. Check before invoicing.",
			InputSchema: schema([]string{"project"}, periodProps, map[string]any{
				"project": map[string]any{"type": "string"},
			}),
			Annotations: readOnly,
			Call: func(raw json.RawMessage) (any, error) {
				var a struct {
					periodArgs
					Project string `json:"project"`
				}
				if err := decode(raw, &a); err != nil {
					return nil, err
				}
				if a.Project == "" {
					return nil, fmt.Errorf("project is required")
				}
				if a.Period == "" && a.LastDays == 0 && a.Since == "" {
					a.Period = "month"
				}
				from, to, err := a.resolve(e.Now())
				if err != nil {
					return nil, err
				}
				return e.with(func(s db.Store) (any, error) {
					days, err := ledger.LoadUnlogged(s, from, to, a.Project)
					if err != nil {
						return nil, err
					}
					if days == nil {
						days = []ledger.UnloggedDay{}
					}
					return map[string]any{"project": a.Project, "from": from, "to": to, "days": days}, nil
				})
			},
		},
		{
			Name:  "sync",
			Title: "Import agent sessions",
			Description: "Import new AI agent sessions (Claude Code, Codex, Hermes) and the user's commits into btrack. " +
				"Safe to repeat: existing sessions are refreshed, never duplicated.",
			InputSchema: schema(nil, map[string]any{
				"sources": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"since":   map[string]any{"type": "string", "description": "Only sessions started on or after YYYY-MM-DD."},
			}),
			Annotations: map[string]any{"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true, "openWorldHint": false},
			Call: func(raw json.RawMessage) (any, error) {
				var a struct {
					Sources []string `json:"sources"`
					Since   string   `json:"since"`
				}
				if err := decode(raw, &a); err != nil {
					return nil, err
				}
				var since time.Time
				if a.Since != "" {
					var err error
					if since, err = time.ParseInLocation("2006-01-02", a.Since, time.Local); err != nil {
						return nil, fmt.Errorf("since must be YYYY-MM-DD")
					}
				}
				return e.with(func(s db.Store) (any, error) {
					return ingest.Run(s, ingest.RunOptions{Home: e.Home, Sources: a.Sources, Since: since})
				})
			},
		},
		{
			Name:        "projects",
			Title:       "Projects",
			Description: "Every project btrack knows, from the timesheet and from agent sessions, with last activity, plus project aliases.",
			InputSchema: schema(nil),
			Annotations: readOnly,
			Call: func(raw json.RawMessage) (any, error) {
				if err := decode(raw, &struct{}{}); err != nil {
					return nil, err
				}
				return e.with(func(s db.Store) (any, error) {
					all, err := s.GetSessionsInRange(time.Time{}, e.Now().Add(24*time.Hour))
					if err != nil {
						return nil, err
					}
					type row struct {
						Name       string    `json:"name"`
						Sessions   int       `json:"sessions"`
						LastActive time.Time `json:"last_active"`
						Sources    []string  `json:"sources"`
					}
					rows := map[string]*row{}
					for _, x := range all {
						if x.Project == "" {
							continue
						}
						r, ok := rows[x.Project]
						if !ok {
							r = &row{Name: x.Project}
							rows[x.Project] = r
						}
						r.Sessions++
						if x.StartTime.After(r.LastActive) {
							r.LastActive = x.StartTime
						}
						found := false
						for _, src := range r.Sources {
							found = found || src == x.Source
						}
						if !found {
							r.Sources = append(r.Sources, x.Source)
						}
					}
					out := make([]row, 0, len(rows))
					for _, r := range rows {
						sort.Strings(r.Sources)
						out = append(out, *r)
					}
					sort.Slice(out, func(i, j int) bool { return out[i].LastActive.After(out[j].LastActive) })
					aliases, err := s.GetAliases()
					if err != nil {
						return nil, err
					}
					return map[string]any{"projects": out, "aliases": aliases}, nil
				})
			},
		},
	}
}

func sessionView(s *db.Session) map[string]any {
	v := map[string]any{
		"id": s.ID, "task": s.TaskName, "project": s.Project, "source": s.Source,
		"start": s.StartTime, "end": s.EndTime, "wall_seconds": int64(s.Duration().Seconds()),
	}
	if s.Message != "" {
		v["message"] = s.Message
	}
	if s.ActiveSeconds != nil {
		v["you_seconds"] = *s.ActiveSeconds
	}
	if s.AgentSeconds != nil {
		v["agent_seconds"] = *s.AgentSeconds
	}
	if s.Model != "" {
		v["model"] = s.Model
	}
	if s.CostUSD != nil {
		v["cost_usd_estimate"] = *s.CostUSD
	}
	return v
}

// parseTime reads HH:MM (on ref's date), "YYYY-MM-DD HH:MM", or RFC 3339.
func parseTime(s string, ref time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("15:04", s); err == nil {
		r := ref.Local()
		return time.Date(r.Year(), r.Month(), r.Day(), t.Hour(), t.Minute(), 0, 0, time.Local), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised time %q: use HH:MM, 'YYYY-MM-DD HH:MM' or RFC 3339", s)
}
