package ui

import (
	"context"
	"path/filepath"

	"github.com/brizzai/fleet/internal/debuglog"
	"github.com/brizzai/fleet/internal/git"
	"github.com/brizzai/fleet/internal/notify"
	"github.com/brizzai/fleet/internal/session"
)

// notifyState is this process's system-notification machinery. Both halves are
// nil in tests that build a Home literal, which turns notifications off.
type notifyState struct {
	tracker  *notify.Tracker
	notifier *notify.Notifier
}

func (h *Home) initNotify() {
	h.notify = notifyState{tracker: notify.NewTracker(), notifier: notify.New()}
}

// maybeNotify runs on the status worker after every status pass over a session
// and raises a system notification when the pass moved it from working to
// waiting / finished / error. Every session counts, whatever the area filter
// hides; the one the user is attached to does not. The blocking part (pane
// capture, transcript, the delivery itself) runs on its own goroutine, so the
// worker never waits on it.
func (h *Home) maybeNotify(s *session.Session, from, to session.Status) {
	if h.notify.tracker == nil {
		return
	}
	enabled := h.cfg != nil && h.cfg.IsNotificationsEnabled()
	var on []string
	if enabled {
		on = h.cfg.NotifyOn()
	}
	h.workerMu.Lock()
	attached := h.attachedSessionID == s.ID
	h.workerMu.Unlock()
	// Observe even when off: it is what keeps the first pass a baseline, so
	// switching notifications on later does not announce old news.
	state := h.notify.tracker.Observe(s.ID, string(from), string(to), attached, on)
	if state == "" || !enabled {
		return
	}

	ev := notify.Event{
		SessionID: s.ID,
		State:     state,
		Title:     s.Title,
		Sound:     h.cfg.IsNotifySound(),
	}
	account, _ := h.accountBadge(s)
	areaName := ""
	if h.areaEnabled() {
		areaName = h.areaOfPath(s.ProjectPath)
	}
	root, branch := git.ReadHead(s.ProjectPath)
	repo := ""
	if root != "" {
		repo = filepath.Base(root)
	}
	ev.Subtitle = notify.Subtitle(areaName, account, repo, branch)
	hookStatus, hookAt := s.GetHookStatus(), s.GetHookUpdatedAt()

	n := h.notify.notifier
	go func() {
		defer func() {
			if r := recover(); r != nil {
				debuglog.Logger.Error("notify panic recovered", "panic", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 2*notify.Timeout)
		defer cancel()
		ev.Message = notify.Message(s, state)
		ev.Key = notify.TransitionKey(state, hookStatus, hookAt, ev.Message)
		sent, mech, err := n.Send(ctx, ev)
		switch {
		case err != nil:
			debuglog.Logger.Error("notify failed", "id", s.ID, "state", state, "mechanism", mech, "err", err)
		case sent:
			debuglog.Logger.Info("notified", "id", s.ID, "state", state, "mechanism", mech)
		}
	}()
}
