// Package ingest turns AI coding agent session logs (Claude Code, Codex,
// Hermes, ...) into btrack sessions. Each source has a Reader that parses
// its own on-disk format into the neutral Session type below; everything
// after that (time split, cost, project resolution, storage) is shared.
package ingest

import (
	"sort"
	"time"
)

// EventKind says who produced a timeline event.
type EventKind int

const (
	// Human is a prompt the user typed (not a tool result, not a
	// system/meta message injected by the tool).
	Human EventKind = iota
	// Agent is anything the agent produced: an assistant message, a tool
	// call, or a tool result arriving.
	Agent
)

// Event is one timestamped point in a session's timeline.
type Event struct {
	At   time.Time
	Kind EventKind
}

// ModelCall is the token usage of one model request, kept per call so a
// session that switched models is priced correctly.
type ModelCall struct {
	Model      string
	Input      int64 // uncached input tokens
	Output     int64
	CacheRead  int64
	CacheWrite int64
}

// Session is one agent session as parsed from a source, before it is
// mapped to a db.Session.
type Session struct {
	Source     string // stable source id, e.g. "claude-code", "codex", "hermes"
	ExternalID string // the tool's own session id; must be stable across re-reads
	Title      string // best-effort human title: first real prompt or tool-provided title
	Cwd        string // working directory recorded by the tool, absolute
	GitBranch  string // if the tool recorded it
	// PathHints are absolute paths the agent touched (tool arguments,
	// tool-reported working directories). When Cwd is the home directory
	// they decide which project the session belonged to.
	PathHints []string
	Events    []Event
	Calls     []ModelCall
	// CostUSD is set only when the tool itself recorded a cost; otherwise
	// the shared pricing table estimates it from Calls.
	CostUSD *float64
}

// Start and End are the first and last event times.
func (s *Session) Start() time.Time { return s.Events[0].At }
func (s *Session) End() time.Time   { return s.Events[len(s.Events)-1].At }

// Reader parses one source.
type Reader interface {
	// Source returns the stable source id stored in db.Session.Source.
	Source() string
	// Discover returns the files or databases to read, given the user's
	// home directory. Missing directories are not an error: return nil.
	Discover(home string) ([]string, error)
	// Read parses one discovered path. It may return zero or many
	// sessions. Sessions with fewer than one Human event are dropped by
	// the caller, so readers need not filter them.
	Read(path string) ([]Session, error)
}

// Split is how a session's wall time divides between a human at the
// keyboard and the agent working alone. Time neither party plausibly
// spent (idle gaps) is in neither bucket, so Active+Agent <= Wall.
type Split struct {
	Wall   time.Duration
	Active time.Duration
	Agent  time.Duration
}

// SplitOptions tunes the active-time heuristic.
type SplitOptions struct {
	// IdleGap: a pause longer than this means nobody was working through
	// it. Default 10m.
	IdleGap time.Duration
	// ReturnAllowance: when the human returns after an idle gap, credit
	// this much for reading output and typing the prompt. Default 2m.
	ReturnAllowance time.Duration
	// AgentStall: a gap between two agent events longer than this is
	// treated as a stall (rate limit, hung tool, laptop asleep) and not
	// counted. Default 30m.
	AgentStall time.Duration
}

func (o SplitOptions) withDefaults() SplitOptions {
	if o.IdleGap == 0 {
		o.IdleGap = 10 * time.Minute
	}
	if o.ReturnAllowance == 0 {
		o.ReturnAllowance = 2 * time.Minute
	}
	if o.AgentStall == 0 {
		o.AgentStall = 30 * time.Minute
	}
	return o
}

// ComputeSplit attributes every gap between consecutive events:
//
//   - gap ending in a Human event: the human was reading the previous
//     output and typing. Credited to Active, capped at IdleGap; a longer
//     gap means they walked away, so only ReturnAllowance is credited.
//     The first prompt of a session also gets ReturnAllowance.
//   - gap ending in an Agent event: the agent was working. Credited to
//     Agent unless it exceeds AgentStall.
//
// It is a heuristic and labelled as such in every report.
func ComputeSplit(events []Event, opt SplitOptions) Split {
	opt = opt.withDefaults()
	if len(events) == 0 {
		return Split{}
	}
	ev := append([]Event(nil), events...)
	sort.SliceStable(ev, func(i, j int) bool { return ev[i].At.Before(ev[j].At) })

	var sp Split
	sp.Wall = ev[len(ev)-1].At.Sub(ev[0].At)
	if ev[0].Kind == Human {
		// Reading and typing the first prompt happened before its timestamp.
		sp.Active += opt.ReturnAllowance
		sp.Wall += opt.ReturnAllowance
	}
	for i := 1; i < len(ev); i++ {
		gap := ev[i].At.Sub(ev[i-1].At)
		if gap <= 0 {
			continue
		}
		switch ev[i].Kind {
		case Human:
			if gap <= opt.IdleGap {
				sp.Active += gap
			} else {
				sp.Active += opt.ReturnAllowance
			}
		case Agent:
			if gap <= opt.AgentStall {
				sp.Agent += gap
			}
		}
	}
	return sp
}

// HumanTurns counts Human events.
func HumanTurns(events []Event) int {
	n := 0
	for _, e := range events {
		if e.Kind == Human {
			n++
		}
	}
	return n
}
