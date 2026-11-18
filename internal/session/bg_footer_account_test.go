package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const footerOnlyAgentPane = "⏺ Pronto.\n\n✻ Cogitated for 1h 48m · done 9:20 AM\n\n────────────────────────────────────────\n❯ \n────────────────────────────────────────\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · ← 1 agent\n"

const waitingLinePane = "⏺ Aguardando a revisão.\n\n✻ Waiting for 1 background agent to finish\n\n────────────────────────────────────────\n❯ \n────────────────────────────────────────\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · ← 1 agent\n"

func TestBackgroundAgentsFooterOnly(t *testing.T) {
	rule := "────────────────"
	cases := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"counter alone", []string{"  ⏵⏵ bypass permissions on (shift+tab to cycle) · ← 1 agent", rule, "❯ ", rule}, true},
		{"counter with waiting spinner line", []string{"  ⏵⏵ bypass permissions on · ← 1 agent", rule, "❯ ", rule, "✻ Waiting for 1 background agent to finish"}, false},
		{"counter with agents still running summary", []string{"  ⏵⏵ bypass permissions on · ← 1 agent", rule, "❯ ", rule, "✻ Churned for 2m · done 3:03 PM · 1 agent still running"}, false},
		{"counter with dock rows", []string{"  ◯ general-purpose  Lendo rotas   35s · ↓ 93.8k tokens", "  ⏺ main", "  ⏵⏵ bypass permissions on · ← 1 agent · ↓ to manage"}, false},
		{"shells only", []string{"  ⏵⏵ bypass permissions on · 1 shell · ↓ to manage", rule, "❯ ", rule}, false},
		{"nothing", []string{"  ⏵⏵ bypass permissions on (shift+tab to cycle)", rule, "❯ ", rule}, false},
	}
	for _, c := range cases {
		if got := backgroundAgentsFooterOnly(c.lines); got != c.want {
			t.Errorf("%s: backgroundAgentsFooterOnly = %v, want %v", c.name, got, c.want)
		}
	}
}

// The "← N agent" counter is per account: after the Stop hook, with no subagent of THIS
// session writing its transcript, it must not keep the session green.
func TestScenarioFooterCounterWithoutOwnSubagentSettles(t *testing.T) {
	runScenario(t, Scenario{
		Name:            "finished hook + footer-only counter + no live subagent: finished",
		SubagentsActive: func() bool { return false },
		Events:          []ScenarioEvent{{At: 0, Hook: "finished", SessionID: "owner-aaa", HookAge: 20 * time.Minute, Pane: footerOnlyAgentPane}},
		Checks:          []ScenarioCheck{{At: 0, Expected: StatusFinished}},
	})
}

// A subagent of this session that is still writing keeps the lead Running.
func TestScenarioFooterCounterWithOwnSubagentStaysRunning(t *testing.T) {
	runScenario(t, Scenario{
		Name:            "finished hook + footer-only counter + live subagent: running",
		SubagentsActive: func() bool { return true },
		Events:          []ScenarioEvent{{At: 0, Hook: "finished", SessionID: "owner-aaa", HookAge: 20 * time.Minute, Pane: footerOnlyAgentPane}},
		Checks:          []ScenarioCheck{{At: 0, Expected: StatusRunning}},
	})
}

// The lead's own spinner line is session-specific evidence: it pins Running even with
// no subagent file (the lookup is never consulted).
func TestScenarioWaitingLineStaysRunningWithoutSubagentFiles(t *testing.T) {
	runScenario(t, Scenario{
		Name:            "finished hook + Waiting for background agent line: running",
		SubagentsActive: func() bool { return false },
		Events:          []ScenarioEvent{{At: 0, Hook: "finished", SessionID: "owner-aaa", HookAge: 20 * time.Minute, Pane: waitingLinePane}},
		Checks:          []ScenarioCheck{{At: 0, Expected: StatusRunning}},
	})
}

// The real lookup: a recent subagent transcript counts, an old or absent one does not,
// and a session whose transcript can't be found keeps the old pinned behaviour.
func TestSubagentsActiveWithinReadsTranscriptDir(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	s := &Session{ID: "x", Agent: "claude", ClaudeSessionID: "abc", ProjectPath: "/tmp/proj-subagents-test"}

	if !s.subagentsActiveWithin(time.Minute) {
		t.Fatal("no transcript on disk: want true (unverifiable keeps the pin)")
	}
	tp := ClaudeTranscriptPath("abc", "/tmp/proj-subagents-test")
	if tp == "" {
		t.Skip("transcript path not resolvable in this environment")
	}
	if err := os.MkdirAll(filepath.Dir(tp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tp, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s.subagentsActiveWithin(time.Minute) {
		t.Fatal("transcript without subagents dir: want false")
	}
	sub := filepath.Join(filepath.Dir(tp), "abc", "subagents")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(sub, "agent-1.jsonl")
	if err := os.WriteFile(f, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !s.subagentsActiveWithin(time.Minute) {
		t.Fatal("fresh subagent transcript: want true")
	}
	old := time.Now().Add(-10 * time.Minute)
	if err := os.Chtimes(f, old, old); err != nil {
		t.Fatal(err)
	}
	if s.subagentsActiveWithin(time.Minute) {
		t.Fatal("stale subagent transcript: want false")
	}
}
