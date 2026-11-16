package tmux

import (
	"path/filepath"
	"strings"
	"testing"
)

// A deleted project folder must fail loudly before any tmux session is created,
// not leave a pane at a shell prompt in a dead directory.
func TestStartRefusesMissingWorkDir(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "apagada")
	s := NewSession("sem-pasta", gone)
	err := s.Start("echo oi")
	if err == nil {
		_ = s.Kill()
		t.Fatal("Start succeeded for a directory that does not exist")
	}
	if !strings.Contains(err.Error(), "no longer exists") {
		t.Errorf("error should name the missing directory, got: %v", err)
	}
	if s.Exists() {
		_ = s.Kill()
		t.Error("a tmux session was created despite the missing directory")
	}
}
