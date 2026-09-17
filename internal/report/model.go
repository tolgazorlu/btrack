// Package report builds a client-facing work report: what was committed and
// how long it took, for a date range, as a self-contained HTML sheet.
package report

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Kind classifies a commit for the coloured tag in the report.
type Kind string

const (
	KindFeat   Kind = "feat"
	KindFix    Kind = "fix"
	KindChore  Kind = "chore"
	KindRevert Kind = "revert"
)

// Label is the Turkish text printed inside the tag chip.
func (k Kind) Label() string {
	switch k {
	case KindFeat:
		return "feat"
	case KindFix:
		return "fix"
	case KindRevert:
		return "geri al"
	default:
		return "bakım"
	}
}

// CSSClass maps the kind onto the stylesheet's tag classes.
func (k Kind) CSSClass() string {
	switch k {
	case KindFeat:
		return "tag-feat"
	case KindFix:
		return "tag-fix"
	case KindRevert:
		return "tag-revert"
	default:
		return "tag-chore"
	}
}

// Commit is one merged change in the reporting window.
type Commit struct {
	SHA     string
	Date    time.Time
	Kind    Kind
	Subject string // the human sentence shown to the client
	PR      int    // 0 when the commit carries no PR reference
	Areas   []string
	Files   []string
}

// ShortSHA is the 7-character form used in the entry meta line.
func (c Commit) ShortSHA() string {
	if len(c.SHA) > 7 {
		return c.SHA[:7]
	}
	return c.SHA
}

// Entry is one stopwatch record: a duration plus what it was spent on.
type Entry struct {
	Date        time.Time
	Duration    time.Duration
	Description string
	Flag        string // optional chip: "commit yok", "Selge", ...
}

// Day groups everything that happened on one calendar day.
type Day struct {
	Date    time.Time
	Commits []Commit
	Entries []Entry
}

// Weekday returns the Turkish weekday name.
func (d Day) Weekday() string {
	return turkishWeekdays[d.Date.Weekday()]
}

// FormatDate renders the day as 21.08.2026.
func (d Day) FormatDate() string {
	return d.Date.Format("02.01.2006")
}

// Areas is the de-duplicated union of every commit's areas, in first-seen order.
func (d Day) Areas() string {
	seen := map[string]bool{}
	var out []string
	for _, c := range d.Commits {
		for _, a := range c.Areas {
			if a == "" || seen[a] {
				continue
			}
			seen[a] = true
			out = append(out, a)
		}
	}
	return strings.Join(out, " · ")
}

// CommitCount summarises the day's commits and PR span, e.g. "6 commit · PR #139–#141".
func (d Day) CommitCount() string {
	if len(d.Commits) == 0 {
		return ""
	}
	label := fmt.Sprintf("%d commit", len(d.Commits))
	prs := d.prRange()
	if prs == "" {
		return label
	}
	return label + " · " + prs
}

func (d Day) prRange() string {
	var nums []int
	seen := map[int]bool{}
	for _, c := range d.Commits {
		if c.PR == 0 || seen[c.PR] {
			continue
		}
		seen[c.PR] = true
		nums = append(nums, c.PR)
	}
	if len(nums) == 0 {
		return ""
	}
	sort.Ints(nums)
	if len(nums) == 1 {
		return fmt.Sprintf("PR #%d", nums[0])
	}
	return fmt.Sprintf("PR #%d–#%d", nums[0], nums[len(nums)-1])
}

// Total is the summed stopwatch time for the day.
func (d Day) Total() time.Duration {
	var sum time.Duration
	for _, e := range d.Entries {
		sum += e.Duration
	}
	return sum
}

// TotalLabel renders the day total as "1 sa 33 dk 56 sn".
func (d Day) TotalLabel() string { return FormatDuration(d.Total()) }

// TotalSub renders the secondary line, "1,57 sa · 4 kayıt".
func (d Day) TotalSub() string {
	return fmt.Sprintf("%s sa · %d kayıt", FormatDecimalHours(d.Total()), len(d.Entries))
}

// Report is everything the template needs.
type Report struct {
	Title       string
	Client      string
	Subtitle    string
	From        time.Time
	To          time.Time
	Days        []Day // ordered ascending; carries both commits and entries
	Reconcile   []string
	Pending     []string
	PreparedBy  string
	GeneratedAt time.Time

	// Figures
	FilesChanged int
	LinesAdded   int
	LinesRemoved int
}

// Range renders "21 Ağustos – 16 Eylül 2026".
func (r Report) Range() string {
	from := fmt.Sprintf("%d %s", r.From.Day(), turkishMonths[r.From.Month()])
	to := fmt.Sprintf("%d %s %d", r.To.Day(), turkishMonths[r.To.Month()], r.To.Year())
	if r.From.Year() != r.To.Year() {
		from = fmt.Sprintf("%s %d", from, r.From.Year())
	}
	return from + " – " + to
}

// ShortRange renders "21.08 – 16.09".
func (r Report) ShortRange() string {
	return r.From.Format("02.01") + " – " + r.To.Format("02.01")
}

// CommitDays are the days that carry at least one commit.
func (r Report) CommitDays() []Day {
	var out []Day
	for _, d := range r.Days {
		if len(d.Commits) > 0 {
			out = append(out, d)
		}
	}
	return out
}

// HourDays are the days that carry at least one stopwatch entry.
func (r Report) HourDays() []Day {
	var out []Day
	for _, d := range r.Days {
		if len(d.Entries) > 0 {
			out = append(out, d)
		}
	}
	return out
}

// TotalCommits counts every commit in the window.
func (r Report) TotalCommits() int {
	n := 0
	for _, d := range r.Days {
		n += len(d.Commits)
	}
	return n
}

// TotalEntries counts every stopwatch record.
func (r Report) TotalEntries() int {
	n := 0
	for _, d := range r.Days {
		n += len(d.Entries)
	}
	return n
}

// TotalDuration sums every stopwatch record.
func (r Report) TotalDuration() time.Duration {
	var sum time.Duration
	for _, d := range r.Days {
		sum += d.Total()
	}
	return sum
}

// WorkedDays counts days with a commit or a stopwatch entry.
func (r Report) WorkedDays() int {
	n := 0
	for _, d := range r.Days {
		if len(d.Commits) > 0 || len(d.Entries) > 0 {
			n++
		}
	}
	return n
}

// PRSpan renders "#138 – #181" across the whole report.
func (r Report) PRSpan() string {
	lo, hi := 0, 0
	for _, d := range r.Days {
		for _, c := range d.Commits {
			if c.PR == 0 {
				continue
			}
			if lo == 0 || c.PR < lo {
				lo = c.PR
			}
			if c.PR > hi {
				hi = c.PR
			}
		}
	}
	if lo == 0 {
		return ""
	}
	if lo == hi {
		return fmt.Sprintf("#%d", lo)
	}
	return fmt.Sprintf("#%d – #%d", lo, hi)
}

// PRCount counts distinct pull requests.
func (r Report) PRCount() int {
	seen := map[int]bool{}
	for _, d := range r.Days {
		for _, c := range d.Commits {
			if c.PR != 0 {
				seen[c.PR] = true
			}
		}
	}
	return len(seen)
}

// TotalLabel renders the grand total as "15 sa 41 dk 02 sn".
func (r Report) TotalLabel() string { return FormatDuration(r.TotalDuration()) }

// TotalDecimal renders the decimal hours, "15,68".
func (r Report) TotalDecimal() string { return FormatDecimalHours(r.TotalDuration()) }

// FormatDuration renders a duration as "1 sa 33 dk 56 sn", dropping the hour
// part when it is zero. Seconds are always shown: the stopwatch records them
// and the client report is expected to reproduce them exactly.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "—"
	}
	total := int(d.Round(time.Second).Seconds())
	h, m, s := total/3600, (total%3600)/60, total%60
	if h > 0 {
		return fmt.Sprintf("%d sa %02d dk %02d sn", h, m, s)
	}
	return fmt.Sprintf("%d dk %02d sn", m, s)
}

// FormatDecimalHours renders decimal hours with a Turkish comma: "15,68".
func FormatDecimalHours(d time.Duration) string {
	return strings.Replace(fmt.Sprintf("%.2f", d.Hours()), ".", ",", 1)
}

var turkishWeekdays = map[time.Weekday]string{
	time.Monday:    "Pazartesi",
	time.Tuesday:   "Salı",
	time.Wednesday: "Çarşamba",
	time.Thursday:  "Perşembe",
	time.Friday:    "Cuma",
	time.Saturday:  "Cumartesi",
	time.Sunday:    "Pazar",
}

var turkishMonths = map[time.Month]string{
	time.January:   "Ocak",
	time.February:  "Şubat",
	time.March:     "Mart",
	time.April:     "Nisan",
	time.May:       "Mayıs",
	time.June:      "Haziran",
	time.July:      "Temmuz",
	time.August:    "Ağustos",
	time.September: "Eylül",
	time.October:   "Ekim",
	time.November:  "Kasım",
	time.December:  "Aralık",
}
