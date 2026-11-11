package ui

import (
	"testing"
	"time"

	"github.com/brizzai/fleet/internal/session"
)

// noInbox switches the inbox off for a tree-shape test that deliberately sets
// sessions Waiting/Finished (NewHome in other tests resets the global).
func noInbox(t *testing.T) {
	t.Helper()
	prev := ShowInbox
	ShowInbox = false
	t.Cleanup(func() { ShowInbox = prev })
}

func inboxSess(id string, st session.Status, accessed time.Time) *session.Session {
	s := &session.Session{ID: id, Title: id, ProjectPath: "/r/" + id, LastAccessedAt: accessed}
	s.SetStatus(st)
	return s
}

func TestAddInbox_LiftsWaitingOnceAndPrunesEmptyOrigin(t *testing.T) {
	now := time.Now()
	waiting := inboxSess("w", session.StatusWaiting, now.Add(-time.Hour))
	running := inboxSess("r", session.StatusRunning, now)
	items := []SidebarItem{
		{IsRepoHeader: true, IsOriginHeader: true, OriginLabel: "alpha", Expanded: true},
		{Session: waiting},
		{IsSpacer: true},
		{IsRepoHeader: true, IsOriginHeader: true, OriginLabel: "beta", Expanded: true},
		{Session: running},
	}
	out := AddInbox(items, true)

	if !out[0].IsSpacer || out[0].Label != "Preciso de você (1)" {
		t.Fatalf("first row must be the inbox title, got %+v", out[0])
	}
	if out[1].Session != waiting || out[1].InboxOrigin != "alpha" {
		t.Fatalf("second row must be the lifted session tagged with its origin, got %+v", out[1])
	}
	count := 0
	for _, it := range out {
		if it.Session == waiting {
			count++
		}
		if it.IsOriginHeader && it.OriginLabel == "alpha" {
			t.Fatalf("origin left empty by the lift must be pruned")
		}
	}
	if count != 1 {
		t.Fatalf("session must appear exactly once, got %d", count)
	}
}

func TestAddInbox_NothingWaitingOrMutedKeepsItems(t *testing.T) {
	running := inboxSess("r", session.StatusRunning, time.Now())
	muted := inboxSess("m", session.StatusWaiting, time.Now())
	items := []SidebarItem{
		{IsRepoHeader: true, IsOriginHeader: true, Expanded: true},
		{Session: running},
		{Session: muted, Snooze: snoozeResult{Muted: true}},
	}
	out := AddInbox(items, true)
	if len(out) != len(items) {
		t.Fatalf("no row qualifies; items must be unchanged, got %d rows", len(out))
	}
}

func TestAddInbox_WaitingBeforeFinishedThenOldestFirst(t *testing.T) {
	now := time.Now()
	fin := inboxSess("fin", session.StatusFinished, now.Add(-3*time.Hour))
	wNew := inboxSess("wnew", session.StatusWaiting, now.Add(-time.Minute))
	wOld := inboxSess("wold", session.StatusWaiting, now.Add(-2*time.Hour))
	items := []SidebarItem{
		{IsRepoHeader: true, IsOriginHeader: true, Expanded: true},
		{Session: fin}, {Session: wNew}, {Session: wOld},
	}
	out := AddInbox(items, true)
	got := []*session.Session{out[1].Session, out[2].Session, out[3].Session}
	want := []*session.Session{wOld, wNew, fin}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pos %d: got %s want %s", i, got[i].ID, want[i].ID)
		}
	}
}
