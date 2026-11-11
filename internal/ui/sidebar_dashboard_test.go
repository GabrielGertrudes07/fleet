package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/brizzai/fleet/internal/session"
	"github.com/brizzai/fleet/internal/stats"
)

func dashFixture() (*stats.Report, []*session.Session) {
	days := make([]stats.Day, 7)
	for i := range days {
		days[i] = stats.Day{CostUSD: float64(i)}
	}
	rep := &stats.Report{
		Days: days,
		Sessions: []stats.SessionUse{
			{ID: "a", Repo: "api", AgentTime: 3*time.Hour + 12*time.Minute, CostUSD: 41.5},
			{ID: "b", Repo: "web", AgentTime: 40 * time.Minute, CostUSD: 4},
			{ID: "gone", Repo: "legacy", AgentTime: 5 * time.Minute},
		},
	}
	ss := []*session.Session{
		{ID: "a", Title: "Ideia à noite — João", Account: "gabriel2@x.com"},
		{ID: "b", Title: "Visual", Account: ""},
	}
	return rep, ss
}

func TestDashboardFullShowsSpendTopSessionsAndAccounts(t *testing.T) {
	rep, ss := dashFixture()
	rows := renderSidebarDashboard(rep, ss, 60, 40)
	out := ansi.Strip(strings.Join(rows, "\n"))
	for _, want := range []string{"Usage · 7d", "today $6.00", "week $21", "Ideia à noite — João", "3h12m", "$42", "legacy", "accounts", "gabriel2 $42"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	for i, r := range rows {
		if w := ansi.StringWidth(r); w != 60 {
			t.Errorf("row %d is %d cols, want exactly 60", i, w)
		}
	}
}

func TestDashboardDegradesWithHeightAndWidth(t *testing.T) {
	rep, ss := dashFixture()
	if got := len(renderSidebarDashboard(rep, ss, 60, 20)); got != 2 {
		t.Errorf("compact form should be 2 rows, got %d", got)
	}
	if renderSidebarDashboard(rep, ss, 60, 10) != nil {
		t.Error("a short sidebar must not carry the dashboard")
	}
	if renderSidebarDashboard(rep, ss, 20, 40) != nil {
		t.Error("a narrow sidebar must not carry the dashboard")
	}
	if renderSidebarDashboard(nil, ss, 60, 40) != nil {
		t.Error("no report yet must render nothing")
	}
}

func TestDashboardNeverOverflowsOnNarrowAndWideTitles(t *testing.T) {
	rep, ss := dashFixture()
	ss[0].Title = strings.Repeat("título muito longo ", 10)
	for _, w := range []int{28, 33, 65} {
		for i, r := range renderSidebarDashboard(rep, ss, w, 40) {
			if got := ansi.StringWidth(r); got != w {
				t.Errorf("width %d row %d is %d cols", w, i, got)
			}
		}
	}
}

func TestDashboardOneAccountHidesAccountRow(t *testing.T) {
	rep, ss := dashFixture()
	ss[1].Account = "gabriel2@x.com"
	out := ansi.Strip(strings.Join(renderSidebarDashboard(rep, ss, 60, 40), "\n"))
	if strings.Contains(out, "accounts") {
		t.Errorf("single account should not show the accounts row:\n%s", out)
	}
}
