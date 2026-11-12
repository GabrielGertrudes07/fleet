package ui

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/brizzai/fleet/internal/config"
	"github.com/brizzai/fleet/internal/notify"
	"github.com/brizzai/fleet/internal/session"
)

type recordingDeliverer struct {
	ch chan notify.Notification
}

func (r recordingDeliverer) Deliver(_ context.Context, n notify.Notification) (string, error) {
	r.ch <- n
	return "fake", nil
}

func notifyTestHome(t *testing.T, enabled bool, on ...string) (*Home, chan notify.Notification) {
	t.Helper()
	cfg := &config.Config{}
	cfg.SetNotificationsEnabled(enabled)
	cfg.Notifications.On = on
	ch := make(chan notify.Notification, 8)
	h := &Home{cfg: cfg, workerMu: sync.Mutex{}}
	h.initAreas()
	h.notify = notifyState{
		tracker:  notify.NewTracker(),
		notifier: &notify.Notifier{Dedup: notify.NewDedup(t.TempDir()), Deliver: recordingDeliverer{ch}},
	}
	return h, ch
}

func expectNone(t *testing.T, ch chan notify.Notification) {
	t.Helper()
	select {
	case n := <-ch:
		t.Fatalf("unexpected notification: %+v", n)
	case <-time.After(150 * time.Millisecond):
	}
}

func TestMaybeNotify(t *testing.T) {
	s := session.NewSession("Fix login", t.TempDir())

	t.Run("startup pass is silent, then running to waiting notifies", func(t *testing.T) {
		h, ch := notifyTestHome(t, true)
		h.maybeNotify(s, session.StatusRunning, session.StatusWaiting) // baseline
		expectNone(t, ch)
		h.maybeNotify(s, session.StatusRunning, session.StatusWaiting)
		select {
		case n := <-ch:
			if n.Title != "Precisa de você · Fix login" || n.Group != "fleet-"+s.ID || n.Sound != "Glass" {
				t.Fatalf("notification = %+v", n)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("no notification")
		}
		h.maybeNotify(s, session.StatusWaiting, session.StatusWaiting)
		expectNone(t, ch)
	})

	t.Run("disabled", func(t *testing.T) {
		h, ch := notifyTestHome(t, false)
		h.maybeNotify(s, session.StatusIdle, session.StatusIdle)
		h.maybeNotify(s, session.StatusRunning, session.StatusWaiting)
		expectNone(t, ch)
	})

	t.Run("state not in on", func(t *testing.T) {
		h, ch := notifyTestHome(t, true, "error")
		h.maybeNotify(s, session.StatusIdle, session.StatusIdle)
		h.maybeNotify(s, session.StatusRunning, session.StatusFinished)
		expectNone(t, ch)
	})

	t.Run("attached session suppressed", func(t *testing.T) {
		h, ch := notifyTestHome(t, true)
		h.maybeNotify(s, session.StatusIdle, session.StatusIdle)
		h.attachedSessionID = s.ID
		h.maybeNotify(s, session.StatusRunning, session.StatusWaiting)
		expectNone(t, ch)
	})
}
