package notify

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brizzai/fleet/internal/session"
)

var allStates = []string{StateWaiting, StateFinished, StateError}

func TestTrackerObserve(t *testing.T) {
	tests := []struct {
		name     string
		prime    bool // a baseline pass happened before
		from, to string
		attached bool
		on       []string
		want     string
	}{
		{"first pass is baseline", false, "running", "finished", false, allStates, ""},
		{"running to waiting", true, "running", "waiting", false, allStates, "waiting"},
		{"starting to error", true, "starting", "error", false, allStates, "error"},
		{"running to finished", true, "running", "finished", false, allStates, "finished"},
		{"waiting stays waiting", true, "waiting", "waiting", false, allStates, ""},
		{"waiting to finished is not from work", true, "waiting", "finished", false, allStates, ""},
		{"finished to idle", true, "finished", "idle", false, allStates, ""},
		{"attached session suppressed", true, "running", "waiting", true, allStates, ""},
		{"state not in on", true, "running", "finished", false, []string{"waiting", "error"}, ""},
		{"state in on", true, "running", "waiting", false, []string{"waiting"}, "waiting"},
		{"running to idle is silent", true, "running", "idle", false, allStates, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := NewTracker()
			if tt.prime {
				tr.Observe("s1", "idle", "idle", false, allStates)
			}
			if got := tr.Observe("s1", tt.from, tt.to, tt.attached, tt.on); got != tt.want {
				t.Fatalf("Observe = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTrackerStartupBurst(t *testing.T) {
	tr := NewTracker()
	// Persisted rows say running; the first pass finds them finished/waiting.
	for _, id := range []string{"a", "b", "c"} {
		if got := tr.Observe(id, "running", "waiting", false, allStates); got != "" {
			t.Fatalf("startup pass for %s notified %q", id, got)
		}
	}
	if got := tr.Observe("a", "running", "finished", false, allStates); got != "finished" {
		t.Fatalf("second pass = %q, want finished", got)
	}
}

func TestFormat(t *testing.T) {
	long := strings.Repeat("palavra ", 60)
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"title waiting", Title(StateWaiting, "Fix login"), "Needs you · Fix login"},
		{"title finished", Title(StateFinished, "Fix login"), "Finished · Fix login"},
		{"title error", Title(StateError, "Fix login"), "Error · Fix login"},
		{"title without session title", Title(StateError, ""), "Error"},
		{"subtitle all", Subtitle("work", "ana", "fleet", "main"), "work · ana · fleet/main"},
		{"subtitle no area/account", Subtitle("", "", "fleet", "main"), "fleet/main"},
		{"subtitle no branch", Subtitle("work", "", "fleet", ""), "work · fleet"},
		{"subtitle empty", Subtitle("", "", "", ""), ""},
		{"ansi stripped", Line("\x1b[31mred\x1b[0m text\x07", 0), "red text"},
		{"newlines collapsed", Line("a\n\n  b\tc", 0), "a b c"},
		{"leading dash dropped", Line("-e evil", 0), "e evil"},
		{"finished first lines", FinishedMessage("\nDone.\n\nChanged 3 files\n"), "Done. Changed 3 files"},
		{"finished empty", FinishedMessage("  "), "Turn finished"},
		{"permission", WaitingMessage(&session.Menu{Kind: "permission", Subject: "Bash(git push origin main)\nmore"}), "Permission: Bash(git push origin main)"},
		{"question with options", WaitingMessage(&session.Menu{Kind: "question", Question: "Which DB?", Options: []session.MenuOption{{N: 1, Label: "Postgres"}, {N: 2, Label: "SQLite"}}}), "Which DB? [Postgres / SQLite]"},
		{"no menu", WaitingMessage(nil), "Waiting for your input"},
		{"error from pane", ErrorMessage([]string{"x", "You've hit your 5-hour limit · resets 12:10pm", "", "> "}, ""), "You've hit your 5-hour limit · resets 12:10pm"},
		{"error fallback", ErrorMessage([]string{"hello"}, ""), "Session stopped"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("got %q, want %q", tt.got, tt.want)
			}
		})
	}

	got := Line(long, 0)
	if n := len([]rune(got)); n > MaxMessage || !strings.HasSuffix(got, "…") {
		t.Fatalf("not truncated: %d runes, %q", n, got)
	}
}

type fakeDeliverer struct {
	mu   sync.Mutex
	got  []Notification
	err  error
	mech string
}

func (f *fakeDeliverer) Deliver(_ context.Context, n Notification) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, n)
	return f.mech, f.err
}

func TestDedupTwoNotifiersOneDelivery(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeDeliverer{mech: "fake"}
	a := &Notifier{Dedup: NewDedup(dir), Deliver: fake}
	b := &Notifier{Dedup: NewDedup(dir), Deliver: fake}
	ev := Event{SessionID: "abc-123", State: StateWaiting, Key: "h100", Title: "t", Message: "m", Sound: true}

	var delivered atomic.Int32
	var wg sync.WaitGroup
	for _, n := range []*Notifier{a, b, a, b} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sent, _, err := n.Send(context.Background(), ev); err != nil {
				t.Error(err)
			} else if sent {
				delivered.Add(1)
			}
		}()
	}
	wg.Wait()
	if delivered.Load() != 1 || len(fake.got) != 1 {
		t.Fatalf("delivered %d, fake got %d; want 1", delivered.Load(), len(fake.got))
	}

	ev.Key = "h200" // the next transition is a new claim
	if sent, _, _ := a.Send(context.Background(), ev); !sent {
		t.Fatal("new key must notify")
	}
	if fake.got[0].Group != "fleet-abc-123" || fake.got[0].Sound != SoundWaiting {
		t.Fatalf("notification = %+v", fake.got[0])
	}
}

func TestDedupSweepRemovesOldClaims(t *testing.T) {
	dir := t.TempDir()
	d := NewDedup(dir)
	if _, err := d.Claim("s", "waiting", "old"); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-25 * time.Hour)
	if err := os.Chtimes(dir+"/"+claimName("s", "waiting", "old"), old, old); err != nil {
		t.Fatal(err)
	}
	d2 := NewDedup(dir)
	if _, err := d2.Claim("s", "waiting", "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir + "/" + claimName("s", "waiting", "old")); !os.IsNotExist(err) {
		t.Fatalf("old claim survived the sweep: %v", err)
	}
	if ok, _ := d2.Claim("s", "waiting", "new"); ok {
		t.Fatal("fresh claim must survive")
	}
}

func TestClaimNameIsOneSafePathElement(t *testing.T) {
	n := claimName("../x/y", "waiting", "a b/c")
	if strings.ContainsAny(n, "/ ") {
		t.Fatalf("unsafe name %q", n)
	}
}

func TestSendDeliveryErrorStillConsumesClaim(t *testing.T) {
	fake := &fakeDeliverer{err: errors.New("boom"), mech: "fake"}
	n := &Notifier{Dedup: NewDedup(t.TempDir()), Deliver: fake}
	sent, mech, err := n.Send(context.Background(), Event{SessionID: "s", State: StateError, Key: "k"})
	if sent || err == nil || mech != "fake" {
		t.Fatalf("sent=%v mech=%q err=%v", sent, mech, err)
	}
}

func TestTransitionKey(t *testing.T) {
	at := time.Unix(1700000000, 0)
	if got := TransitionKey(StateWaiting, "waiting", at, "anything"); got != "h1700000000" {
		t.Fatalf("hook-driven key = %q", got)
	}
	if got := TransitionKey(StateError, "running", at, "x"); got != "h1700000000" {
		t.Fatalf("error key = %q", got)
	}
	p1 := TransitionKey(StateWaiting, "running", at, "prompt one")
	p2 := TransitionKey(StateWaiting, "running", at, "prompt two")
	if p1 == p2 || p1 != TransitionKey(StateWaiting, "running", at, "prompt one") {
		t.Fatalf("pane-led keys must be stable per message and differ across messages: %q %q", p1, p2)
	}
}

func found(string) (string, error)    { return "/bin/x", nil }
func notFound(string) (string, error) { return "", errors.New("no") }

func TestOsascriptArgsNeverInterpolateText(t *testing.T) {
	nasty := `"; do shell script "touch /tmp/pwned" --`
	n := Notification{Title: nasty, Subtitle: `x" & quit`, Message: nasty, Sound: SoundWaiting}
	cmd, ok := BuildCommand("darwin", func(name string) (string, error) {
		if name == "osascript" {
			return "/usr/bin/osascript", nil
		}
		return "", errors.New("no")
	}, n)
	if !ok || cmd.Mechanism != MechOsascript || cmd.Name != "osascript" {
		t.Fatalf("cmd = %+v", cmd)
	}
	var script string
	for i, a := range cmd.Args {
		if a == "-e" {
			script = cmd.Args[i+1]
		}
	}
	for _, bad := range []string{"pwned", "do shell script", "quit", n.Title, n.Message, n.Subtitle} {
		if strings.Contains(script, bad) {
			t.Fatalf("script contains user text %q:\n%s", bad, script)
		}
	}
	if !strings.Contains(script, "item 1 of argv") || !strings.Contains(script, `sound name "Glass"`) {
		t.Fatalf("script = %s", script)
	}
	tail := cmd.Args[len(cmd.Args)-4:]
	if tail[0] != "--" || tail[1] != n.Message || tail[2] != n.Title || tail[3] != n.Subtitle {
		t.Fatalf("texts must follow -- as argv, got %q", tail)
	}
}

func TestBuildCommandMechanisms(t *testing.T) {
	n := Notification{Title: "T", Subtitle: "S", Message: "M", Group: "fleet-1", Sound: SoundDefault, BundleID: "com.apple.Terminal"}
	tests := []struct {
		name string
		goos string
		look func(string) (string, error)
		mech string
		ok   bool
	}{
		{"darwin prefers terminal-notifier", "darwin", found, MechTerminalNotifier, true},
		{"darwin falls back to osascript", "darwin", func(s string) (string, error) {
			if s == "osascript" {
				return "x", nil
			}
			return "", errors.New("no")
		}, MechOsascript, true},
		{"darwin none", "darwin", notFound, "", false},
		{"linux notify-send", "linux", found, MechNotifySend, true},
		{"linux none", "linux", notFound, "", false},
		{"windows unsupported", "windows", found, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, ok := BuildCommand(tt.goos, tt.look, n)
			if ok != tt.ok || cmd.Mechanism != tt.mech {
				t.Fatalf("got (%q, %v), want (%q, %v)", cmd.Mechanism, ok, tt.mech, tt.ok)
			}
		})
	}

	cmd, _ := BuildCommand("darwin", found, n)
	want := []string{"-title", "T", "-message", "M", "-group", "fleet-1", "-subtitle", "S", "-sound", "default", "-activate", "com.apple.Terminal"}
	if strings.Join(cmd.Args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("terminal-notifier args = %q", cmd.Args)
	}
	n.Sound, n.BundleID, n.Subtitle = "", "", ""
	cmd, _ = BuildCommand("darwin", found, n)
	for _, a := range cmd.Args {
		if a == "-sound" || a == "-activate" || a == "-subtitle" {
			t.Fatalf("empty option leaked: %q", cmd.Args)
		}
	}
}

func TestBundleID(t *testing.T) {
	tests := map[string]string{
		"Apple_Terminal": "com.apple.Terminal",
		"iTerm.app":      "com.googlecode.iterm2",
		"ghostty":        "com.mitchellh.ghostty",
		"WezTerm":        "com.github.wez.wezterm",
		"tmux":           "",
		"":               "",
	}
	for in, want := range tests {
		if got := BundleID(in); got != want {
			t.Errorf("BundleID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSoundFor(t *testing.T) {
	if SoundFor(StateWaiting, true) != "Glass" || SoundFor(StateFinished, true) != "default" || SoundFor(StateError, false) != "" {
		t.Fatal("sound mapping wrong")
	}
}
