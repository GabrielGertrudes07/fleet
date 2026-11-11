package notify

import (
	"github.com/brizzai/fleet/internal/agent"
	"github.com/brizzai/fleet/internal/session"
)

// Message reads what the notification should say from the live session: the
// prompt it waits on, its last reply, or the failure line. It does blocking
// I/O (a pane capture, a transcript tail) and belongs off the UI goroutine.
func Message(s *session.Session, state string) string {
	a := agent.Parse(string(s.Agent))
	capture := ""
	if ts := s.GetTmuxSession(); ts != nil {
		capture, _ = ts.CapturePane()
	}
	switch state {
	case StateWaiting:
		var menu *session.Menu
		if capture != "" {
			menu = session.ParsePaneView(a, capture).Menu
		}
		return WaitingMessage(menu)
	case StateFinished:
		if a == agent.Claude {
			path := session.ClaudeTranscriptPath(s.GetClaudeSessionID(), s.ProjectPath)
			if turns, _ := session.ReadClaudeTurns(path, 1); len(turns) > 0 {
				if r := turns[len(turns)-1].Reply; r != "" {
					return FinishedMessage(r)
				}
			}
		}
		return FinishedMessage("")
	default:
		var lines []string
		if capture != "" {
			lines = session.CleanPaneLines(capture)
		}
		return ErrorMessage(lines, "")
	}
}
