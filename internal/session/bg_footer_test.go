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
