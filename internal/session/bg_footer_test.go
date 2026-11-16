package session

import "testing"

func TestDetectBackgroundAgentsInFooter(t *testing.T) {
	// recentLines are newest-first (index 0 = bottom of the pane).
	rule := "────────────────"
	cases := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"footer bypass 1 agent", []string{"  ⏵⏵ bypass permissions on (shift+tab to cycle) · ← 1 agent", rule, "❯ ", rule, "✻ Crunched for 6m 49s · done 2:00 PM"}, true},
		{"footer with shell and manage hint", []string{"  ⏵⏵ bypass permissions on · 1 shell · ← 2 agents · ↓ to manage", rule, "❯ ", rule}, true},
		{"waiting spinner line", []string{rule, "❯ ", rule, "✻ Waiting for 1 background agent to finish"}, true},
		{"footer shells only", []string{"  ⏵⏵ bypass permissions on (shift+tab to cycle) · 2 shells · ↓ to manage", rule, "❯ ", rule}, true},
		{"still running summary line", []string{rule, "❯ ", rule, "", "✻ Churned for 49m 11s · done 3:03 PM · 2 shells still running"}, true},
		{"prose mentioning shells still running", []string{rule, "❯ ", rule, "  deixei 2 shells still running no servidor"}, false},
		{"waiting line with another spinner glyph", []string{rule, "❯ ", rule, "✶ Waiting for 2 background agents to finish"}, true},
		{"waiting line quoted without glyph", []string{rule, "❯ ", rule, "  a linha Waiting for 1 background agent aparece"}, false},
		{"summary 24h clock", []string{rule, "❯ ", rule, "", "✻ Churned for 1m · done 15:03 · 1 shell still running"}, true},
		{"stale still-running line under a newer finished summary", []string{rule, "❯ ", rule, "✻ Brewed for 3s · done 3:09 PM", "o shell terminou", "✻ Churned for 49m · done 3:03 PM · 2 shells still running"}, false},
		{"no agents", []string{"  ⏵⏵ bypass permissions on (shift+tab to cycle)", rule, "❯ ", rule}, false},
		{"quoted in conversation, no chrome", []string{rule, "❯ ", rule, "  o rodapé mostra ← 1 agent quando há agentes"}, false},
		{"zero agents", []string{"  ⏵⏵ bypass permissions on · ← 0 agents", rule, "❯ "}, false},
		{"quoted far above chrome", []string{"a", "b", "c", "d", "e", "f", "  ⏵⏵ bypass · ← 1 agent"}, false},
	}
	for _, c := range cases {
		if got := detectBackgroundAgentsRunning(c.lines); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
