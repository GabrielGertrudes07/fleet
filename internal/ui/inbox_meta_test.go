package ui

import (
	"testing"
	"time"

	"github.com/brizzai/fleet/internal/session"
)

func TestFilterByTagAndStampMeta(t *testing.T) {
	a := inboxSess("a", session.StatusIdle, time.Now())
	b := inboxSess("b", session.StatusIdle, time.Now())
	h := &Home{meta: map[string]session.Meta{a.ID: {Tag: "folha", Note: "n"}}}

	got, f := h.filterByTag([]*session.Session{a, b}, "#fol")
	if len(got) != 1 || got[0] != a || f != "" {
		t.Fatalf("#fol must keep only the tagged session and clear the title filter, got %d, %q", len(got), f)
	}
	got, f = h.filterByTag([]*session.Session{a, b}, "texto")
	if len(got) != 2 || f != "texto" {
		t.Fatalf("a plain filter must pass through untouched")
	}

	items := []SidebarItem{{Session: a}, {Session: b}, {IsSpacer: true}}
	h.stampMeta(items)
	if items[0].Tag != "folha" || items[0].Note != "n" || items[1].Tag != "" {
		t.Fatalf("stampMeta wrong: %+v", items)
	}
}
