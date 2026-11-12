package notify

import (
	"github.com/brizzai/fleet/internal/agent"
	"github.com/brizzai/fleet/internal/session"
)

// Message reads what the notification should say from the live session: the
// prompt it waits on, its last reply, or the failure line. It does blocking
// I/O (a pane capture, a transcript tail) and belongs off the UI goroutine.
//
// It returns the effective state and whether to announce at all: with
// needsMarker a finished turn is announced only when the reply closes with
// "result:", "failed:" (finished) or "needs input:" (promoted to waiting);
// a progress update stays silent. Without a readable reply (non-Claude agent)
// it announces as before.
func Message(s *session.Session, state string, needsMarker bool) (msg, eff string, ok bool) {
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
		return WaitingMessage(menu), state, true
	case StateFinished:
		if a == agent.Claude {
			path := session.ClaudeTranscriptPath(s.GetClaudeSessionID(), s.ProjectPath)
			if turns, _ := session.ReadClaudeTurns(path, 1); len(turns) > 0 {
				if r := turns[len(turns)-1].Reply; r != "" {
					switch Marker(r) {
					case MarkerNeedsInput:
						return FinishedMessage(r), StateWaiting, true
					case MarkerDone:
						return FinishedMessage(r), state, true
					}
					return FinishedMessage(r), state, !needsMarker
				}
			}
		}
		return FinishedMessage(""), state, true
	default:
		var lines []string
		if capture != "" {
			lines = session.CleanPaneLines(capture)
		}
		return ErrorMessage(lines, ""), state, true
	}
}
