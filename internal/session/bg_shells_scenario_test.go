package session

import (
	"testing"
	"time"
)

const shellsOnlyPane = "⏺ Aguardando o build.\n\n✻ Churned for 4m 2s · done 3:03 PM · 1 shell still running\n\n────────────────────────────────────────\n❯ \n────────────────────────────────────────\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 1 shell · ↓ to manage\n"

const shellAndAgentPane = "⏺ Aguardando.\n\n✻ Churned for 4m 2s · done 3:03 PM\n\n────────────────────────────────────────\n❯ \n────────────────────────────────────────\n  ⏵⏵ bypass permissions on (shift+tab to cycle) · 1 shell · ← 1 agent · ↓ to manage\n"

// A lead parked on a background shell is busy for a while (the shell will notify it)...
func TestScenarioBackgroundShellRecentStaysRunning(t *testing.T) {
	runScenario(t, Scenario{
		Name: "finished hook 5m ago + background shell: running",
		Events: []ScenarioEvent{
			{At: 0, Hook: "finished", SessionID: "owner-aaa", HookAge: 5 * time.Minute, Pane: shellsOnlyPane},
		},
		Checks: []ScenarioCheck{{At: 0, Expected: StatusRunning}},
	})
}

// ...but a shell that outlives backgroundShellsOnlyPinnedFor is a dev server, not work.
func TestScenarioBackgroundShellOldSettles(t *testing.T) {
	runScenario(t, Scenario{
		Name: "finished hook 31m ago + background shell only: settles to finished",
		Events: []ScenarioEvent{
			{At: 0, Hook: "finished", SessionID: "owner-aaa", HookAge: 31 * time.Minute, Pane: shellsOnlyPane},
		},
		Checks: []ScenarioCheck{{At: 0, Expected: StatusFinished}},
	})
}

// Agents re-invoke the lead, so they pin running however long they take.
func TestScenarioBackgroundAgentOldStaysRunning(t *testing.T) {
	runScenario(t, Scenario{
		Name: "finished hook 31m ago + background agent: still running",
		Events: []ScenarioEvent{
			{At: 0, Hook: "finished", SessionID: "owner-aaa", HookAge: 31 * time.Minute, Pane: shellAndAgentPane},
		},
		Checks: []ScenarioCheck{{At: 0, Expected: StatusRunning}},
	})
}
