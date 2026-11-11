package stats

import (
	"testing"
	"time"
)

func TestSessionUsageJoinsTimeAndCostAndRanks(t *testing.T) {
	d := &dataset{sessions: map[string]sessionInfo{
		"a": {agent: "claude", repo: "api"},
		"b": {agent: "codex", repo: "web"},
	}}
	per := map[string][]ival{
		"a": {{s: 0, e: 60_000}},
		"b": {{s: 0, e: 600_000}},
	}
	// 1M output tokens of claude-fable-5-1 = $50 at the table's rate.
	turns := []turnData{{session: "a", model: "claude-fable-5-1", out: 1_000_000}}

	got := sessionUsage(d, per, turns)
	if len(got) != 2 {
		t.Fatalf("want 2 sessions, got %d: %+v", len(got), got)
	}
	if got[0].ID != "b" || got[0].AgentTime != 10*time.Minute {
		t.Errorf("rank 0 should be b with 10m, got %+v", got[0])
	}
	if got[1].ID != "a" || got[1].CostUSD < 49.9 || got[1].CostUSD > 50.1 || got[1].Repo != "api" {
		t.Errorf("a should carry ~$50 and repo api, got %+v", got[1])
	}
}

func TestSessionUsageDropsEmptyAndDefaultsAgent(t *testing.T) {
	d := &dataset{sessions: map[string]sessionInfo{}}
	// Unknown model: tokens count, dollars do not -> nothing to rank.
	got := sessionUsage(d, nil, []turnData{{session: "x", model: "nope", out: 10}})
	if len(got) != 0 {
		t.Fatalf("a session with no time and no priced cost must be dropped: %+v", got)
	}
	got = sessionUsage(d, map[string][]ival{"y": {{s: 0, e: 1000}}}, nil)
	if len(got) != 1 || got[0].Agent != "claude" {
		t.Fatalf("legacy session defaults to claude: %+v", got)
	}
}

func TestDayCostsBucketsByLocalDay(t *testing.T) {
	d1 := time.Date(2026, 11, 10, 23, 0, 0, 0, time.Local)
	d2 := time.Date(2026, 11, 11, 1, 0, 0, 0, time.Local)
	m := dayCosts([]turnData{
		{start: d1.UnixMilli(), model: "claude-fable-5-1", out: 1_000_000},
		{start: d2.UnixMilli(), model: "claude-fable-5-1", out: 2_000_000},
	})
	if m["2026-11-10"] < 49 || m["2026-11-11"] < 99 {
		t.Fatalf("wrong buckets: %v", m)
	}
}
