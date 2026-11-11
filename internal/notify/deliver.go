package notify

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Notification is one ready-to-show message.
type Notification struct {
	Title, Subtitle, Message string
	// Group replaces an earlier notification with the same group
	// (terminal-notifier only): one live notification per session.
	Group string
	// Sound is a macOS sound name, "" for silence.
	Sound string
	// BundleID is the terminal app to focus on click, "" when unknown.
	BundleID string
}

// Sound names. Glass is distinct enough to tell "needs you" from "done".
const (
	SoundWaiting = "Glass"
	SoundDefault = "default"
)

// SoundFor returns the sound for a state, "" when sound is off.
func SoundFor(state string, enabled bool) string {
	switch {
	case !enabled:
		return ""
	case state == StateWaiting:
		return SoundWaiting
	}
	return SoundDefault
}

// Deliverer shows a notification. The returned string names the mechanism used.
// Tests inject a fake so no real notification fires.
type Deliverer interface {
	Deliver(ctx context.Context, n Notification) (string, error)
}

// Mechanisms.
const (
	MechTerminalNotifier = "terminal-notifier"
	MechOsascript        = "osascript"
	MechNotifySend       = "notify-send"
)

// ErrUnsupported means this machine has no way to show a notification.
var ErrUnsupported = errors.New("no notification mechanism available")

// Timeout bounds one delivery.
const Timeout = 5 * time.Second

// Command is an external command line: always argv, never a shell string.
type Command struct {
	Mechanism string
	Name      string
	Args      []string
}

// bundleIDs maps $TERM_PROGRAM to the terminal's bundle identifier, so
// clicking the notification brings that terminal forward.
var bundleIDs = map[string]string{
	"Apple_Terminal": "com.apple.Terminal",
	"iTerm.app":      "com.googlecode.iterm2",
	"ghostty":        "com.mitchellh.ghostty",
	"WezTerm":        "com.github.wez.wezterm",
}

// BundleID returns the bundle id for a TERM_PROGRAM value, "" when unknown.
func BundleID(termProgram string) string { return bundleIDs[termProgram] }

// osascriptSource shows the notification from argv only: the texts are model
// output and must never be spliced into the script. sound is appended from a
// fixed set (SoundFor), never from user text.
const osascriptSource = `on run argv
display notification (item 1 of argv) with title (item 2 of argv) subtitle (item 3 of argv)%s
end run`

// BuildCommand picks the mechanism for goos and builds its command line. ok is
// false when none is available. lookPath is exec.LookPath in production.
func BuildCommand(goos string, lookPath func(string) (string, error), n Notification) (Command, bool) {
	has := func(name string) bool { _, err := lookPath(name); return err == nil }
	switch goos {
	case "darwin":
		if has("terminal-notifier") {
			args := []string{"-title", n.Title, "-message", n.Message, "-group", n.Group}
			if n.Subtitle != "" {
				args = append(args, "-subtitle", n.Subtitle)
			}
			if n.Sound != "" {
				args = append(args, "-sound", n.Sound)
			}
			if n.BundleID != "" {
				args = append(args, "-activate", n.BundleID)
			}
			return Command{MechTerminalNotifier, "terminal-notifier", args}, true
		}
		if has("osascript") {
			sound := ""
			if n.Sound != "" {
				sound = ` sound name "` + soundLiteral(n.Sound) + `"`
			}
			src := sprintfSound(sound)
			// "--" ends osascript's own option parsing so a text starting with
			// "-" is still just an argument.
			return Command{MechOsascript, "osascript", []string{"-e", src, "--", n.Message, n.Title, n.Subtitle}}, true
		}
	case "linux":
		if has("notify-send") {
			body := n.Message
			if n.Subtitle != "" {
				body = n.Subtitle + "\n" + n.Message
			}
			return Command{MechNotifySend, "notify-send", []string{"-a", "fleet", "--", n.Title, body}}, true
		}
	}
	return Command{}, false
}

// soundLiteral keeps the sound name inside the script to a safe alphabet; the
// callers only pass SoundFor's constants, this is the belt to those braces.
func soundLiteral(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == ' ' {
			out = append(out, r)
		}
	}
	return string(out)
}

func sprintfSound(sound string) string {
	return strings.Replace(osascriptSource, "%s", sound, 1)
}

// SystemDeliverer shows notifications through the OS.
type SystemDeliverer struct{}

// Deliver implements Deliverer. When terminal-notifier is installed but fails
// (typically macOS has not granted it notification permission), it falls back
// to osascript, and reports both errors if that fails too.
func (SystemDeliverer) Deliver(ctx context.Context, n Notification) (string, error) {
	n.BundleID = firstNonEmpty(n.BundleID, TerminalBundleID())
	mech, err := runCommand(ctx, runtime.GOOS, exec.LookPath, n)
	if err != nil && mech == MechTerminalNotifier {
		skip := func(name string) (string, error) {
			if name == "terminal-notifier" {
				return "", errors.New("skipped")
			}
			return exec.LookPath(name)
		}
		if mech2, err2 := runCommand(ctx, runtime.GOOS, skip, n); err2 == nil {
			return mech2, nil
		} else if mech2 != "" {
			return mech2, errors.Join(err, err2)
		}
	}
	return mech, err
}

func runCommand(ctx context.Context, goos string, lookPath func(string) (string, error), n Notification) (string, error) {
	cmd, ok := BuildCommand(goos, lookPath, n)
	if !ok {
		return "", ErrUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	if out, err := exec.CommandContext(ctx, cmd.Name, cmd.Args...).CombinedOutput(); err != nil {
		return cmd.Mechanism, errors.Join(err, errors.New(Line(string(out), 200)))
	}
	return cmd.Mechanism, nil
}

// TerminalBundleID is the bundle id of the terminal fleet runs in, from the
// environment ("" when unknown).
func TerminalBundleID() string {
	return BundleID(os.Getenv("TERM_PROGRAM"))
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
