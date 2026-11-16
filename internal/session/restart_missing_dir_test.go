package session

import (
	"path/filepath"
	"strings"
	"testing"
)

// Restarting into a project folder that no longer exists must fail before anything is
// killed or marked, so the old pane (scrollback, exit reason) survives the refusal.
func TestRestartRefusesMissingProjectDirBeforeTouchingState(t *testing.T) {
	s := &Session{ID: "abc12345-1", Title: "sem pasta", ProjectPath: filepath.Join(t.TempDir(), "apagada"), Status: StatusError}
	err := s.Restart()
	if err == nil || !strings.Contains(err.Error(), "no longer exists") {
		t.Fatalf("want a 'no longer exists' error, got %v", err)
	}
	if s.GetStatus() != StatusError {
		t.Errorf("status changed to %s; a refused restart must leave state alone", s.GetStatus())
	}
}
