// Package notify raises a system notification when a session stops or needs
// the user: transition detection, message formatting, cross-instance dedup and
// delivery (terminal-notifier / osascript on macOS, notify-send on Linux).
package notify

import (
	"strings"
	"unicode"

	"github.com/brizzai/fleet/internal/session"
)

// Notification states. They match config.NotifyStates.
const (
	StateWaiting  = "waiting"
	StateFinished = "finished"
	StateError    = "error"
)

// Title prefixes, one per state. Kept together so the wording is easy to find
// and change (the project is upstream-English; the labels are not localized).
const (
	titleWaiting  = "Needs you"
	titleFinished = "Finished"
	titleError    = "Error"
)

// MaxMessage is the longest message, in runes, a notification carries.
const MaxMessage = 220

// Title is "<label> · <session title>".
func Title(state, sessionTitle string) string {
	label := titleFinished
	switch state {
	case StateWaiting:
		label = titleWaiting
	case StateError:
		label = titleError
	}
	sessionTitle = Line(sessionTitle, 80)
	if sessionTitle == "" {
		return label
	}
	return label + " · " + sessionTitle
}

// Subtitle joins the non-empty parts with " · ": area, account, "<repo>/<branch>".
func Subtitle(area, account, repo, branch string) string {
	var parts []string
	for _, p := range []string{area, account} {
		if p = Line(p, 40); p != "" {
			parts = append(parts, p)
		}
	}
	switch rb := joinRepoBranch(repo, branch); rb {
	case "":
	default:
		parts = append(parts, rb)
	}
	return strings.Join(parts, " · ")
}

func joinRepoBranch(repo, branch string) string {
	repo, branch = Line(repo, 40), Line(branch, 60)
	switch {
	case repo != "" && branch != "":
		return repo + "/" + branch
	case repo != "":
		return repo
	}
	return branch
}

// Line turns arbitrary text (often LLM or terminal output) into one clean
// line: ANSI and control characters removed, whitespace collapsed, leading
// bullet/dash characters dropped (a leading "-" would read as a flag to the
// delivery tools), truncated to max runes with an ellipsis. max <= 0 means
// MaxMessage.
func Line(s string, max int) string {
	if max <= 0 {
		max = MaxMessage
	}
	s = session.StripANSI(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			return ' '
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimLeft(s, "-–—•*> ")
	r := []rune(s)
	if len(r) > max {
		s = strings.TrimSpace(string(r[:max-1])) + "…"
	}
	return s
}

// WaitingMessage says what the session is asking for, from the prompt on its
// screen: "Permission: <command>" for a tool approval, otherwise the question
// followed by its options. nil means the pane showed no parsable prompt.
func WaitingMessage(m *session.Menu) string {
	if m == nil {
		return "Waiting for your input"
	}
	if m.Kind == "permission" {
		what := m.Subject
		if what == "" && len(m.Context) > 0 {
			what = m.Context[len(m.Context)-1]
		}
		if what == "" {
			what = m.Question
		}
		return Line("Permission: "+firstLine(what), 0)
	}
	var opts []string
	for _, o := range m.Options {
		opts = append(opts, Line(o.Label, 40))
	}
	msg := firstLine(m.Question)
	if msg == "" && len(m.Context) > 0 {
		msg = firstLine(m.Context[len(m.Context)-1])
	}
	if len(opts) > 0 {
		var b strings.Builder
		for i, o := range opts {
			if i > 0 {
				b.WriteString(" / ")
			}
			b.WriteString(o)
		}
		msg += " [" + b.String() + "]"
	}
	if strings.TrimSpace(msg) == "" {
		return "Waiting for your input"
	}
	return Line(msg, 0)
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			return l
		}
	}
	return ""
}

// FinishedMessage is the opening of the agent's last reply: its first non-empty
// lines joined, cut to MaxMessage.
func FinishedMessage(reply string) string {
	var parts []string
	for _, l := range strings.Split(reply, "\n") {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "```") {
			parts = append(parts, t)
			if len(parts) == 3 {
				break
			}
		}
	}
	if len(parts) == 0 {
		return "Turn finished"
	}
	return Line(strings.Join(parts, " "), 0)
}

// errorMarkers are lowercase fragments that mark a pane line as the failure
// reason (usage limit, API error, crash).
var errorMarkers = []string{"limit", "error", "failed", "exceeded", "overloaded", "unauthorized", "rate", "resets", "exited", "denied"}

// ErrorMessage picks the failure line from the pane's cleaned lines, looking
// from the bottom up. fallback is used when nothing matches.
func ErrorMessage(lines []string, fallback string) string {
	for i, seen := len(lines)-1, 0; i >= 0 && seen < 40; i-- {
		t := strings.TrimSpace(lines[i])
		if t == "" {
			continue
		}
		seen++
		low := strings.ToLower(t)
		for _, m := range errorMarkers {
			if strings.Contains(low, m) {
				return Line(t, 0)
			}
		}
	}
	if fallback == "" {
		fallback = "Session stopped"
	}
	return Line(fallback, 0)
}
