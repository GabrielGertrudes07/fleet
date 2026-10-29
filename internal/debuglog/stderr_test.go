package debuglog

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The child inherits fd 2 at fork time; the parent's redirect must catch what
// it writes there, and restore must hand fd 2 back to the original target.
func TestRedirectStderr(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	origPath := filepath.Join(t.TempDir(), "orig")
	orig, err := os.Create(origPath)
	if err != nil {
		t.Fatal(err)
	}
	defer orig.Close()
	realStderr := os.Stderr
	os.Stderr = orig
	defer func() { os.Stderr = realStderr }()

	restore := RedirectStderr()
	cmd := exec.Command("sh", "-c", "echo child-noise >&2")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	os.Stderr.WriteString("parent-noise\n")
	restore()
	restore() // idempotent
	os.Stderr.WriteString("after-restore\n")

	got, _ := os.ReadFile(StderrPath())
	if !strings.Contains(string(got), "child-noise") || !strings.Contains(string(got), "parent-noise") {
		t.Fatalf("stderr.log = %q, want child and parent noise", got)
	}
	if strings.Contains(string(got), "after-restore") {
		t.Fatalf("stderr.log got a write made after restore: %q", got)
	}
	back, _ := os.ReadFile(origPath)
	if string(back) != "after-restore\n" {
		t.Fatalf("original stderr = %q, want only the post-restore write", back)
	}
}
