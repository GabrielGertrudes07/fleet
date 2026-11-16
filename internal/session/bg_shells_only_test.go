package session

import "testing"

func TestBackgroundShellsOnly(t *testing.T) {
	rule := "────────────────"
	cases := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"shells in footer", []string{"  ⏵⏵ bypass permissions on · 1 shell · ↓ to manage", rule, "❯ ", rule}, true},
		{"still-running summary naming shells", []string{rule, "❯ ", rule, "✻ Churned for 49m · done 3:03 PM · 2 shells still running"}, true},
		{"shell and agent together is not shells-only", []string{"  ⏵⏵ bypass permissions on · 1 shell · ← 1 agent · ↓ to manage", rule, "❯ ", rule}, false},
		{"agent only", []string{"  ⏵⏵ bypass permissions on (shift+tab to cycle) · ← 1 agent", rule, "❯ ", rule}, false},
		{"dock rows present", []string{"  ◯ general-purpose  Lendo rotas   35s · ↓ 93.8k tokens", "  ⏺ main", "  ⏵⏵ bypass permissions on · 1 shell · ↓ to manage", rule}, false},
		{"summary naming agents is not shells-only", []string{rule, "❯ ", rule, "✻ Churned for 2m · done 3:03 PM · 1 agent still running"}, false},
		{"nothing in flight", []string{"  ⏵⏵ bypass permissions on (shift+tab to cycle)", rule, "❯ ", rule}, false},
	}
	for _, c := range cases {
		if got := backgroundShellsOnly(c.lines); got != c.want {
			t.Errorf("%s: backgroundShellsOnly = %v, want %v", c.name, got, c.want)
		}
	}
}

// Everything that counts as work in flight still reads Running; only the shells-only
// age cap (applied in updateStatusFromHook) lets a long-lived shell stop pinning.
func TestBackgroundWorkSplitsAgentsAndShells(t *testing.T) {
	rule := "────────────────"
	a, s := backgroundWork([]string{"  ⏵⏵ bypass permissions on · 2 shells · ← 1 agent · ↓ to manage", rule})
	if !a || !s {
		t.Errorf("footer with both: agents=%v shells=%v, want both true", a, s)
	}
	a, s = backgroundWork([]string{rule, "✶ Waiting for 1 background agent to finish"})
	if !a || s {
		t.Errorf("waiting line: agents=%v shells=%v, want agents only", a, s)
	}
}
