package stats

import (
	"sort"
	"time"
)

// SessionUse is one fleet session's footprint in the report window: the data
// behind the sidebar dashboard's "most time" and "most spend" rankings. The ID
// is the fleet session id; the UI resolves it to a title, because stats.db
// deliberately never stores titles.
type SessionUse struct {
	ID        string
	Repo      string
	Agent     string
	AgentTime time.Duration
	// CostUSD is the API-list-price equivalent of the session's tokens, the
	// same basis as Hero.CostUSD — not what a subscription actually bills.
	CostUSD float64
	Tokens  int64
}

// sessionUsage joins agent time (per) with token cost (turns) by session,
// keeping only sessions that used either. Sorted by agent time, then cost.
func sessionUsage(d *dataset, per map[string][]ival, turns []turnData) []SessionUse {
	by := map[string]*SessionUse{}
	get := func(sid string) *SessionUse {
		u := by[sid]
		if u == nil {
			si := d.sessions[sid]
			agent := si.agent
			if agent == "" {
				agent = "claude"
			}
			u = &SessionUse{ID: sid, Repo: si.repo, Agent: agent}
			by[sid] = u
		}
		return u
	}
	for sid, iv := range per {
		if ms := ivalsTotal(iv); ms > 0 {
			get(sid).AgentTime = time.Duration(ms) * time.Millisecond
		}
	}
	for _, t := range turns {
		u := get(t.session)
		u.Tokens += t.tokens()
		if c, ok := turnCost(t.model, t.in, t.out, t.cr, t.cw5, t.cw1); ok {
			u.CostUSD += c
		}
	}
	out := make([]SessionUse, 0, len(by))
	for _, u := range by {
		if u.AgentTime > 0 || u.CostUSD > 0 {
			out = append(out, *u)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].AgentTime != out[j].AgentTime {
			return out[i].AgentTime > out[j].AgentTime
		}
		if out[i].CostUSD != out[j].CostUSD {
			return out[i].CostUSD > out[j].CostUSD
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// dayCosts sums turn cost per local day (by the turn's start), keyed by dayKey.
func dayCosts(turns []turnData) map[string]float64 {
	m := map[string]float64{}
	for _, t := range turns {
		if c, ok := turnCost(t.model, t.in, t.out, t.cr, t.cw5, t.cw1); ok {
			m[dayKey(time.UnixMilli(t.start))] += c
		}
	}
	return m
}
