package notify

import (
	"slices"
	"sync"
)

// Tracker decides which status changes deserve a notification. It is fed every
// status change the worker sees, from every session (including ones hidden by
// the area filter), and answers whether to notify.
type Tracker struct {
	mu   sync.Mutex
	seen map[string]bool
}

// NewTracker returns an empty Tracker.
func NewTracker() *Tracker { return &Tracker{seen: map[string]bool{}} }

func isWorking(status string) bool { return status == "running" || status == "starting" }

// Observe records one status pass over a session (from → to, which may be
// equal) and returns the notification state ("waiting", "finished", "error")
// when this pass is a transition worth announcing, else "".
//
// The first pass over a session in this process is only a baseline: status
// rows persisted by a previous run get reconciled with reality on it, and
// announcing those would be a burst at every startup. After that a transition
// counts only when it comes from a working state (running/starting) into a
// state listed in on — so waiting→waiting never repeats, and finished→idle or
// error→error are silent. attached is the session the user is sitting in on
// this instance; they are already looking at it.
func (t *Tracker) Observe(id, from, to string, attached bool, on []string) string {
	t.mu.Lock()
	first := !t.seen[id]
	t.seen[id] = true
	t.mu.Unlock()
	if first || attached || from == to || !isWorking(from) {
		return ""
	}
	switch to {
	case StateWaiting, StateFinished, StateError:
		if slices.Contains(on, to) {
			return to
		}
	}
	return ""
}

// Forget drops a removed session.
func (t *Tracker) Forget(id string) {
	t.mu.Lock()
	delete(t.seen, id)
	t.mu.Unlock()
}
