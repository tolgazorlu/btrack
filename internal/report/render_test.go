package report

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func render(t *testing.T, r Report) string {
	t.Helper()
	var b bytes.Buffer
	if err := r.Render(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestPart3OnlyWhenRequestedAndCostOnlyWithFlag(t *testing.T) {
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	base := Report{
		Client: "Acme", From: day, To: day,
		Days: Build(nil, []Entry{{Date: day.Add(9 * time.Hour), Duration: time.Hour, Description: "call"}}),
	}
	if out := render(t, base); strings.Contains(out, "Bölüm 3") {
		t.Error("part 3 rendered without agent data")
	}

	withAgents := base
	withAgents.Agent = &AgentWork{Sessions: 2, Linked: 3, Days: []AgentDay{
		{Date: day, You: 28 * time.Minute, Agent: time.Hour, CostUSD: 4.5, CostComplete: true, Sessions: 2},
	}}
	out := render(t, withAgents)
	for _, want := range []string{"Bölüm 3", "2 ajan oturumu", "3 commit bir oturuma bağlı", "28 dk 00 sn", "1 sa 00 dk 00 sn", "eklenmez"} {
		if !strings.Contains(out, want) {
			t.Errorf("part 3 missing %q", want)
		}
	}
	if strings.Contains(out, "~$") {
		t.Error("cost shown without ShowCost")
	}

	withAgents.Agent.ShowCost = true
	if out := render(t, withAgents); !strings.Contains(out, "~$4,50") {
		t.Error("cost missing with ShowCost")
	}
}

func TestFormatUSD(t *testing.T) {
	if got := FormatUSD(12.4, true); got != "~$12,40" {
		t.Errorf("got %q", got)
	}
	if got := FormatUSD(0, false); got != "~$0,00+" {
		t.Errorf("got %q", got)
	}
}

func TestRateCalculatorUsesExactSeconds(t *testing.T) {
	day := time.Date(2026, 10, 27, 0, 0, 0, 0, time.Local)
	r := Report{
		Client: "Acme", From: day, To: day,
		Days: Build(nil, []Entry{
			{Date: day.Add(9 * time.Hour), Duration: 9*time.Minute + 29*time.Second, Description: "fix"},
			{Date: day.Add(10 * time.Hour), Duration: 30 * time.Minute, Description: "feature"},
		}),
	}
	out := render(t, r)
	for _, want := range []string{`data-seconds="2369"`, `id="fee-rate"`, "Saatlik ücret", "Toplam tutar"} {
		if !strings.Contains(out, want) {
			t.Errorf("calculator missing %q", want)
		}
	}
}
