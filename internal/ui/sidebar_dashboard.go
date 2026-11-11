package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/brizzai/fleet/internal/session"
	"github.com/brizzai/fleet/internal/stats"
)

// The sidebar dashboard: a few fixed rows under the session list with the last
// 7 days' spend, a spend sparkline, the sessions that held an agent busy the
// longest, and spend per account. It is fed by the same stats.Report as the
// Stats screen (`i`), so every number here is local and the dollars carry the
// same caveat: API-list-price equivalent, not what a subscription bills.

const (
	// dashFullMinHeight / dashCompactMinHeight are the sidebar inner heights
	// below which the dashboard drops to its compact form, then disappears, so
	// the session list is never squeezed on a short terminal.
	dashFullMinHeight    = 28
	dashCompactMinHeight = 18
	dashTopSessions      = 3
	dashTopAccounts      = 3
	// statsDashRefresh is how often the footer's report is recomputed while
	// fleet is open; the scan completing refreshes it sooner.
	statsDashRefresh = 5 * time.Minute
)

var dashSparkBlocks = []rune("▁▂▃▄▅▆▇█")

// dashAccountOf names a session's account for the per-account row: the part of
// the email before "@", or "default" for the ambient login.
func dashAccountOf(s *session.Session) string {
	if s == nil || s.Account == "" {
		return "default"
	}
	if i := strings.IndexByte(s.Account, '@'); i > 0 {
		return s.Account[:i]
	}
	return s.Account
}

func dashMoney(v float64) string {
	switch {
	case v <= 0:
		return "$0"
	case v < 10:
		return fmt.Sprintf("$%.2f", v)
	case v < 1000:
		return fmt.Sprintf("$%.0f", v)
	default:
		return fmt.Sprintf("$%.1fk", v/1000)
	}
}

func dashDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

// dashSpark draws vals as one block per value, scaled to the largest. The
// shade comes from the Stats data ramp — the sanctioned exception to "colour
// means status" (docs/design-system.md §5): it is a magnitude scale.
func dashSpark(vals []float64) string {
	var peak float64
	for _, v := range vals {
		peak = max(peak, v)
	}
	var b strings.Builder
	for _, v := range vals {
		if peak <= 0 || v <= 0 {
			b.WriteString(lipgloss.NewStyle().Foreground(ColorBorder).Render(string(dashSparkBlocks[0])))
			continue
		}
		f := v / peak
		idx := min(int(f*float64(len(dashSparkBlocks)-1)+0.5), len(dashSparkBlocks)-1)
		shade := 1 + int(f*float64(statsRampN-2)+0.5)
		b.WriteString(StatsRamp[min(shade, statsRampN-1)].Render(string(dashSparkBlocks[idx])))
	}
	return b.String()
}

// dashLine fits one styled line to width, padding the right edge.
func dashLine(s string, width int) string {
	if ansi.StringWidth(s) > width {
		s = ansi.Truncate(s, width, "…")
	}
	if pad := width - ansi.StringWidth(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// renderSidebarDashboard returns the dashboard rows for a sidebar width wide
// and height tall, or nil when it would crowd the list. sessions resolves ids
// to titles and accounts; a session no longer in fleet shows as its repo.
func renderSidebarDashboard(rep *stats.Report, sessions []*session.Session, width, height int) []string {
	if rep == nil || width < 28 || height < dashCompactMinHeight {
		return nil
	}
	dim := lipgloss.NewStyle().Foreground(ColorTextDim)
	txt := lipgloss.NewStyle().Foreground(ColorText)
	strong := lipgloss.NewStyle().Foreground(ColorText).Bold(true)

	// Last 7 local days of spend, oldest first; today is the last one.
	days := rep.Days
	if len(days) > 7 {
		days = days[len(days)-7:]
	}
	costs := make([]float64, len(days))
	var week, today float64
	for i, d := range days {
		costs[i] = d.CostUSD
		week += d.CostUSD
	}
	if len(costs) > 0 {
		today = costs[len(costs)-1]
	}

	title := "Usage · 7d"
	rule := dim.Render(strings.Repeat("─", max(width-ansi.StringWidth(title)-3, 0)))
	rows := []string{dashLine(dim.Render("─ ")+strong.Render(title)+" "+rule, width)}

	rows = append(rows, dashLine(
		dim.Render("today ")+strong.Render(dashMoney(today))+
			dim.Render("  week ")+strong.Render(dashMoney(week))+
			"  "+dashSpark(costs)+
			dim.Render("  api est."), width))

	if height < dashFullMinHeight {
		return rows
	}

	byID := make(map[string]*session.Session, len(sessions))
	for _, s := range sessions {
		byID[s.ID] = s
	}

	// Top sessions by agent time.
	shown := 0
	for _, u := range rep.Sessions {
		if shown == dashTopSessions {
			break
		}
		name := u.Repo
		if s := byID[u.ID]; s != nil && s.Title != "" {
			name = s.Title
		}
		if name == "" {
			name = "(session)"
		}
		right := dashDuration(u.AgentTime)
		if u.CostUSD > 0 {
			right += "  " + dashMoney(u.CostUSD)
		}
		w := ansi.StringWidth(right)
		left := ansi.Truncate(name, max(width-w-5, 4), "…")
		gap := max(width-ansi.StringWidth(left)-w-3, 1)
		rows = append(rows, dashLine(
			dim.Render(fmt.Sprintf(" %d ", shown+1))+txt.Render(left)+strings.Repeat(" ", gap)+txt.Render(right), width))
		shown++
	}
	if shown == 0 {
		rows = append(rows, dashLine(dim.Render(" no agent time recorded yet"), width))
	}

	// Spend per account, only when more than one account has spend.
	acct := map[string]float64{}
	for _, u := range rep.Sessions {
		if u.CostUSD > 0 {
			acct[dashAccountOf(byID[u.ID])] += u.CostUSD
		}
	}
	if len(acct) > 1 {
		type kv struct {
			k string
			v float64
		}
		list := make([]kv, 0, len(acct))
		for k, v := range acct {
			list = append(list, kv{k, v})
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].v != list[j].v {
				return list[i].v > list[j].v
			}
			return list[i].k < list[j].k
		})
		if len(list) > dashTopAccounts {
			list = list[:dashTopAccounts]
		}
		parts := make([]string, len(list))
		for i, e := range list {
			parts[i] = txt.Render(e.k) + " " + strong.Render(dashMoney(e.v))
		}
		rows = append(rows, dashLine(dim.Render(" accounts ")+strings.Join(parts, dim.Render(" · ")), width))
	}
	return rows
}
