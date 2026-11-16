package tmux

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartShellArgsEmptyWorkDir(t *testing.T) {
	if got := startShellArgs(""); got != nil {
		t.Errorf("empty workDir must not build a launcher, got %v", got)
	}
}

// The launcher is ONE pre-quoted string (old tmux joins argv with spaces) and must keep
// a path with spaces and a single quote intact.
func TestStartShellArgsIsOneQuotedString(t *testing.T) {
	if _, err := exec.LookPath("perl"); err != nil {
		t.Skip("perl not available")
	}
	dir := filepath.Join(t.TempDir(), "meu projeto d'ouro")
	got := startShellArgs(dir)
	if len(got) != 1 {
		t.Fatalf("want exactly one shell-command string, got %d: %v", len(got), got)
	}
	if !strings.Contains(got[0], shellQuote(dir)) {
		t.Errorf("workDir not shell-quoted in %q", got[0])
	}
}

// Run the launcher string for real through sh -c (what tmux does): the program it execs
// must start with the project directory as its cwd, spaces and quote included.
func TestStartShellArgsLauncherStartsInWorkDir(t *testing.T) {
	if _, err := exec.LookPath("perl"); err != nil {
		t.Skip("perl not available")
	}
	dir := filepath.Join(t.TempDir(), "meu projeto d'ouro")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	launcher := startShellArgs(dir)
	if len(launcher) != 1 {
		t.Fatalf("no launcher: %v", launcher)
	}
	// Replace the trailing `<shell> -l` with a probe that prints its cwd.
	cmdline := strings.TrimSuffix(launcher[0], " -l")
	cmdline = cmdline[:strings.LastIndex(cmdline, " ")] + " /bin/sh -c pwd"
	out, err := exec.Command("/bin/sh", "-c", cmdline).CombinedOutput()
	if err != nil {
		t.Fatalf("launcher failed: %v\n%s", err, out)
	}
	want, _ := filepath.EvalSymlinks(dir)
	got, _ := filepath.EvalSymlinks(strings.TrimSpace(string(out)))
	if got != want {
		t.Errorf("cwd = %q, want %q", got, want)
	}
}

func TestWorkDirMissing(t *testing.T) {
	d := t.TempDir()
	f := filepath.Join(d, "arquivo")
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"existing dir", d, false},
		{"missing", filepath.Join(d, "nada"), true},
		{"a file is not a project dir", f, true},
		// ENOTDIR (a path under a file) is not a clean "does not exist": let the launch speak.
		{"under a file", filepath.Join(f, "x"), false},
	}
	for _, c := range cases {
		if got := workDirMissing(c.path); got != c.want {
			t.Errorf("%s: workDirMissing = %v, want %v", c.name, got, c.want)
		}
	}
}
