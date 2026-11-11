package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/brizzai/fleet/internal/session"
)

// Inbox: sessions that wait on the user (a question, a permission, a finished
// turn nobody read, an error) are lifted out of their origin group into a
// titled section at the top of the sidebar, oldest-waiting first. Like folder
// groups it is a post-pass over BuildFlatItems, so that function and its
// callers are untouched. A session lives in exactly one place — the inbox row
// replaces its tree row (no duplicates, so every "find this session" lookup
// stays unambiguous).

// inboxWants is whether a session row belongs in the inbox. Muted (snoozed)
// sessions stay put: snooze means "not now". Finished only appears while
// unacknowledged — an acknowledged one is already Idle.
func inboxWants(it SidebarItem) bool {
	if it.Session == nil || it.Snooze.Muted {
		return false
	}
	switch it.Session.GetStatus() {
	case session.StatusWaiting, session.StatusFinished, session.StatusError:
		return true
	}
	return false
}

func inboxRank(s *session.Session) int {
	switch s.GetStatus() {
	case session.StatusWaiting:
		return 0
	case session.StatusError:
		return 1
	}
	return 2
}

// HeaderStyleForLabel renders a section title row (the inbox header).
func HeaderStyleForLabel(label string, width int) string {
	return RepoHeaderStyle.Render(truncate(label, width))
}

// AddInbox lifts waiting sessions to the top. With enabled=false, or when
// nothing waits, items come back unchanged.
func AddInbox(items []SidebarItem, enabled bool) []SidebarItem {
	if !enabled {
		return items
	}
	var lifted []SidebarItem
	rest := make([]SidebarItem, 0, len(items))
	originLabel := ""
	for _, it := range items {
		if it.IsFolderHeader {
			originLabel = it.FolderLabel
		} else if it.IsOriginHeader {
			originLabel = it.OriginLabel
		}
		if inboxWants(it) {
			it.InboxOrigin = originLabel
			it.Depth = 0
			it.IsLast = false
			lifted = append(lifted, it)
			continue
		}
		rest = append(rest, it)
	}
	if len(lifted) == 0 {
		return items
	}
	rest = pruneEmptyHeaders(rest)
	sort.SliceStable(lifted, func(i, j int) bool {
		ri, rj := inboxRank(lifted[i].Session), inboxRank(lifted[j].Session)
		if ri != rj {
			return ri < rj
		}
		return lifted[i].Session.LastAccessedAt.Before(lifted[j].Session.LastAccessedAt)
	})
	out := make([]SidebarItem, 0, len(lifted)+len(rest)+2)
	out = append(out, SidebarItem{IsSpacer: true, Label: fmt.Sprintf("Preciso de você (%d)", len(lifted))})
	out = append(out, lifted...)
	if len(rest) > 0 {
		out = append(out, SidebarItem{IsSpacer: true})
		out = append(out, rest...)
	}
	return out
}

func headerLevel(it SidebarItem) int {
	switch {
	case it.IsFolderHeader:
		return 1
	case it.IsOriginHeader:
		return 2
	case it.IsCheckoutHeader:
		return 3
	}
	return 0
}

// pruneEmptyHeaders drops group headers left with no rows underneath once
// sessions were lifted out, and normalizes the spacers between groups.
func pruneEmptyHeaders(items []SidebarItem) []SidebarItem {
	for changed := true; changed; {
		changed = false
		out := make([]SidebarItem, 0, len(items))
		for i, it := range items {
			if l := headerLevel(it); l > 0 {
				hasChild := false
				for j := i + 1; j < len(items); j++ {
					if items[j].IsSpacer {
						continue
					}
					if lj := headerLevel(items[j]); lj > 0 && lj <= l {
						break
					}
					hasChild = true
					break
				}
				// A collapsed header legitimately has no rows below it.
				if !hasChild && it.Expanded {
					changed = true
					continue
				}
			}
			out = append(out, it)
		}
		items = out
	}
	norm := make([]SidebarItem, 0, len(items))
	for _, it := range items {
		if it.IsSpacer && (len(norm) == 0 || norm[len(norm)-1].IsSpacer) {
			continue
		}
		norm = append(norm, it)
	}
	return norm
}

// stampMeta copies each session's user tag and note onto its sidebar row.
func (h *Home) stampMeta(items []SidebarItem) {
	for i := range items {
		if items[i].Session == nil {
			continue
		}
		m := h.meta[items[i].Session.ID]
		items[i].Tag, items[i].Note = m.Tag, m.Note
	}
}

// filterByTag turns a "/#tag" filter into a session subset: the filter text is
// matched against titles by BuildFlatItems, which knows nothing of tags, so a
// leading "#" is resolved here and the title filter cleared.
func (h *Home) filterByTag(sessions []*session.Session, filter string) ([]*session.Session, string) {
	if !strings.HasPrefix(filter, "#") {
		return sessions, filter
	}
	want := strings.ToLower(strings.TrimPrefix(filter, "#"))
	out := make([]*session.Session, 0, len(sessions))
	for _, s := range sessions {
		if strings.Contains(h.meta[s.ID].Tag, want) {
			out = append(out, s)
		}
	}
	return out, ""
}
