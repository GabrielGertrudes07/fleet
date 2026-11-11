package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/brizzai/fleet/internal/agent"
	"github.com/brizzai/fleet/internal/git"
	"github.com/brizzai/fleet/internal/github"
	"github.com/brizzai/fleet/internal/session"
	"github.com/charmbracelet/x/ansi"
)

// setSidebarStyle switches the package global for one test.
func setSidebarStyle(t *testing.T, style string) {
	t.Helper()
	old := SidebarStyle
	SidebarStyle = style
	t.Cleanup(func() { SidebarStyle = old })
}

// shapeAll is shape() plus checkout rows ("C:<basename>"), which shape omits.
func shapeAll(items []SidebarItem) string {
	var out []string
	for _, it := range items {
		d := strings.Repeat(">", it.Depth)
		switch {
		case it.IsSpacer:
			out = append(out, "_")
		case it.IsFolderHeader:
			out = append(out, "F:"+it.FolderLabel)
		case it.IsOriginHeader:
			out = append(out, d+"O:"+it.OriginLabel)
		case it.IsCheckoutHeader:
			out = append(out, d+"C:"+it.RepoPath[strings.LastIndex(it.RepoPath, "/")+1:])
		case it.Session != nil:
			out = append(out, d+"S:"+it.Session.Title)
		}
	}
	return strings.Join(out, " ")
}

type cleanFix struct {
	sessions []*session.Session
	origin   map[string]string
	wt       map[string]bool
}

func (f cleanFix) build(expanded map[string]bool) []SidebarItem {
	return BuildFlatItems(f.sessions, nil, expanded, "", nil, nil, nil, time.Time{},
		func(r string) string { return f.origin[r] },
		func(r string) bool { return f.wt[r] })
}

func newCleanFix(t *testing.T, specs ...[3]string) cleanFix {
	t.Helper()
	f := cleanFix{origin: map[string]string{}, wt: map[string]bool{}}
	for _, sp := range specs { // {title, repo path, origin}
		session.SeedRepoRoot(sp[1], sp[1])
		f.sessions = append(f.sessions, session.NewSession(sp[0], sp[1]))
		f.origin[sp[1]] = sp[2]
	}
	return f
}

func TestCleanStyleShape(t *testing.T) {
	one := newCleanFix(t, [3]string{"s1", "/tmp/cl/lastro", "o/lastro"})
	two := newCleanFix(t,
		[3]string{"a", "/tmp/cl/vessel", "o/vessel"},
		[3]string{"b", "/tmp/cl/vessel-wt", "o/vessel"})
	two.wt["/tmp/cl/vessel-wt"] = true

	tests := []struct {
		name     string
		style    string
		fix      cleanFix
		expanded map[string]bool
		want     string
	}{
		{"classic keeps the checkout row", "classic", one, nil, "O:lastro C:lastro S:s1"},
		{"clean folds the single checkout", "clean", one, nil, "O:lastro S:s1"},
		{"clean collapsed origin hides sessions", "clean", one, map[string]bool{"origin:o/lastro": false}, "O:lastro"},
		{"clean ignores a stale collapsed checkout key", "clean", one, map[string]bool{"/tmp/cl/lastro": false}, "O:lastro S:s1"},
		{"clean keeps rows with two checkouts", "clean", two, nil, "O:vessel C:vessel S:a C:vessel-wt S:b"},
		{"classic two checkouts", "classic", two, nil, "O:vessel C:vessel S:a C:vessel-wt S:b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setSidebarStyle(t, tc.style)
			if got := shapeAll(tc.fix.build(tc.expanded)); got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
		})
	}
}

func TestCleanMergedOriginResolvesCheckout(t *testing.T) {
	setSidebarStyle(t, "clean")
	f := newCleanFix(t, [3]string{"s1", "/tmp/cl/lastro", "o/lastro"})
	items := f.build(nil)
	o := items[0]
	if !o.Merged || o.RepoPath != "/tmp/cl/lastro" {
		t.Fatalf("origin header = Merged:%v RepoPath:%q", o.Merged, o.RepoPath)
	}
	// An origin with 2 checkouts must stay ambiguous: no RepoPath.
	f2 := newCleanFix(t, [3]string{"a", "/tmp/cl/x", "o/x"}, [3]string{"b", "/tmp/cl/x2", "o/x"})
	if o2 := f2.build(nil)[0]; o2.Merged || o2.RepoPath != "" {
		t.Errorf("two-checkout origin must not merge: %+v", o2)
	}
}

func TestCleanHeaderAggregatesOnlyWhenCollapsed(t *testing.T) {
	setSidebarStyle(t, "clean")
	counts := map[session.Status]int{session.StatusError: 1}
	base := SidebarItem{IsRepoHeader: true, IsOriginHeader: true, OriginLabel: "lastro", SessionCount: 1, SessionTotal: 3, StatusCounts: counts}

	tests := []struct {
		name     string
		item     SidebarItem
		style    string
		wantDot  bool
		wantText string
	}{
		{"clean expanded: name only", withExpanded(base, true), "clean", false, "▾ lastro"},
		{"clean collapsed: sessions count + dot", withExpanded(base, false), "clean", true, "▸ lastro 3  ✕"},
		{"classic expanded keeps count + dot", withExpanded(base, true), "classic", true, "▾ lastro 1  ✕"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SidebarStyle = tc.style
			got := ansi.Strip(renderOriginHeader(tc.item, 40, false))
			if got != tc.wantText {
				t.Errorf("got %q want %q", got, tc.wantText)
			}
		})
	}

	// Checkout header (2+ checkouts): pill only when collapsed.
	co := SidebarItem{IsRepoHeader: true, IsCheckoutHeader: true, RepoPath: "/tmp/cl/x", StatusCounts: counts}
	info := &git.RepoInfo{Branch: "main"}
	SidebarStyle = "clean"
	if got := ansi.Strip(renderCheckoutHeader(withExpanded(co, true), info, 40, false)); strings.Contains(got, "✕") {
		t.Errorf("expanded checkout shows a pill: %q", got)
	}
	if got := ansi.Strip(renderCheckoutHeader(withExpanded(co, false), info, 40, false)); !strings.Contains(got, "✕") {
		t.Errorf("collapsed checkout lost its pill: %q", got)
	}
}

func withExpanded(it SidebarItem, e bool) SidebarItem { it.Expanded = e; return it }

func TestCleanInlineBranchAndPR(t *testing.T) {
	setSidebarStyle(t, "clean")
	hdr := SidebarItem{IsRepoHeader: true, IsOriginHeader: true, Merged: true, OriginLabel: "lastro", Expanded: true, RepoPath: "/r"}
	pr := &github.PR{Number: 12, State: "OPEN", CIStatus: "FAILURE", URL: "u"}

	tests := []struct {
		name string
		info *git.RepoInfo
		mut  func(*SidebarItem)
		want string
	}{
		{"branch + dirty", &git.RepoInfo{Branch: "main", IsDirty: true}, nil, "▾ lastro  main*"},
		{"feature branch shows last segment", &git.RepoInfo{Branch: "feat/x"}, nil, "▾ lastro  x"},
		{"PR state stays visible", &git.RepoInfo{Branch: "main", PR: pr}, nil, "▾ lastro  main " + ansi.Strip(renderPRBadge(pr, false))},
		{"no branch: nothing inline", &git.RepoInfo{}, nil, "▾ lastro"},
		{"git info not loaded: nothing inline", nil, nil, "▾ lastro"},
		{"failed removal is not lost", &git.RepoInfo{}, func(i *SidebarItem) { i.RemovalFailed = true }, "▾ lastro  ✕ removal failed — d to retry"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			it := hdr
			if tc.mut != nil {
				tc.mut(&it)
			}
			if got := ansi.Strip(renderOriginHeaderWith(it, tc.info, 60, false)); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
	if !strings.Contains(ansi.Strip(renderPRBadge(pr, false)), "#12") {
		t.Fatal("fixture PR badge has no number")
	}
}

func TestCleanAgentGlyphOnlyWhenMixed(t *testing.T) {
	mk := func(agents ...agent.Type) []SidebarItem {
		var specs [][3]string
		for i := range agents {
			specs = append(specs, [3]string{string(rune('a' + i)), "/tmp/cl/g/r" + string(rune('a'+i)), "o/g" + string(rune('a'+i))})
		}
		f := newCleanFix(t, specs...)
		for i, s := range f.sessions {
			s.Agent = agents[i]
		}
		return f.build(nil)
	}
	glyphShown := func(items []SidebarItem) []bool {
		var out []bool
		for _, it := range items {
			if it.Session != nil {
				out = append(out, strings.Contains(ansi.Strip(renderSessionItem(it, 60, false, -1)), claudeGlyph) ||
					strings.Contains(ansi.Strip(renderSessionItem(it, 60, false, -1)), codexGlyph))
			}
		}
		return out
	}
	tests := []struct {
		name  string
		style string
		ag    []agent.Type
		want  []bool
	}{
		{"clean, one agent: hidden", "clean", []agent.Type{agent.Claude, agent.Claude}, []bool{false, false}},
		{"clean, mixed: shown", "clean", []agent.Type{agent.Claude, agent.Codex}, []bool{true, true}},
		{"classic, one agent: shown", "classic", []agent.Type{agent.Claude, agent.Claude}, []bool{true, true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setSidebarStyle(t, tc.style)
			got := glyphShown(mk(tc.ag...))
			if len(got) != len(tc.want) {
				t.Fatalf("rows = %v", got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("row %d glyph shown = %v, want %v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// With the checkout row folded away, the actions that read "the repo under the
// cursor" must still find it from the origin header.
func TestCleanActionsResolveFromOriginHeader(t *testing.T) {
	setSidebarStyle(t, "clean")
	f := newCleanFix(t, [3]string{"s1", "/tmp/cl/lastro", "o/lastro"})
	h := areaTestHome(t, nil, "", f.sessions...)
	gi := map[string]*git.RepoInfo{"/tmp/cl/lastro": {
		OriginKey: "o/lastro", Branch: "main",
		PR: &github.PR{Number: 7, State: "OPEN", URL: "https://example.test/pr/7"},
	}}
	h.gitInfoCache.Store(&gi)
	h.rebuildFlatItems()
	h.cursor = 0
	if !h.flatItems[0].IsOriginHeader {
		t.Fatalf("row 0 = %+v", h.flatItems[0])
	}
	if got := h.resolveCurrentRepo(); got != "/tmp/cl/lastro" {
		t.Errorf("resolveCurrentRepo = %q", got)
	}
	if got := h.resolveWorktreeBaseRepo(); got != "/tmp/cl/lastro" {
		t.Errorf("resolveWorktreeBaseRepo = %q", got)
	}
	if !h.hasPRForCursor() {
		t.Error("PR actions (p / ctrl+p) not enabled on the origin header")
	}
	if got := h.cursorBarContext(); got != BarContextCheckout {
		t.Errorf("bar context = %v, want checkout keys", got)
	}
	title, items := h.buildContextMenuItems()
	if !strings.HasPrefix(title, "origin: ") {
		t.Errorf("menu title = %q", title)
	}
	ids := map[string]bool{}
	for _, it := range items {
		ids[it.ID] = true
	}
	for _, id := range []string{"new_session", "new_worktree", "branch", "open_pr", "copy_pr_link", "allowed_accounts", "delete_at_cursor"} {
		if !ids[id] {
			t.Errorf("menu lacks %q", id)
		}
	}
}

// The fold composes with folder groups: no checkout rows, members one level in.
func TestCleanStyleInsideFolderGroup(t *testing.T) {
	setSidebarStyle(t, "clean")
	h, _ := folderHome(t, folderAreas, "personal")
	h.rebuildFlatItems()
	got := shapeAll(h.flatItems)
	want := "F:Youtube >O:historia >S:historia >O:lastro >S:lastro _ O:ideias S:ideias"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
