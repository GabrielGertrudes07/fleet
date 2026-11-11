package notify

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Dedup claims a transition so that two fleet instances sharing one state.db
// (`fleet --area work` and `fleet --area home`) announce it once. The claim is
// an O_EXCL file named <session>-<state>-<key>: whoever creates it notifies,
// the other sees it exists and stays quiet.
type Dedup struct {
	Dir string
	// TTL is how long claim files live before the sweep removes them.
	TTL time.Duration

	mu        sync.Mutex
	lastSweep time.Time
	now       func() time.Time
}

// DefaultDir is ~/.config/fleet/notify.
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "fleet", "notify")
}

// NewDedup returns a Dedup over dir with a 24h TTL.
func NewDedup(dir string) *Dedup { return &Dedup{Dir: dir, TTL: 24 * time.Hour, now: time.Now} }

// Claim returns true when this caller is the first to claim the transition.
// An unusable directory returns an error and false: better to miss a
// notification than to risk a duplicate storm.
func (d *Dedup) Claim(sessionID, state, key string) (bool, error) {
	if err := os.MkdirAll(d.Dir, 0o755); err != nil {
		return false, err
	}
	d.maybeSweep()
	f, err := os.OpenFile(filepath.Join(d.Dir, claimName(sessionID, state, key)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, f.Close()
}

func (d *Dedup) maybeSweep() {
	d.mu.Lock()
	if d.now().Sub(d.lastSweep) < time.Hour {
		d.mu.Unlock()
		return
	}
	d.lastSweep = d.now()
	d.mu.Unlock()
	entries, err := os.ReadDir(d.Dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() && d.now().Sub(info.ModTime()) > d.TTL {
			os.Remove(filepath.Join(d.Dir, e.Name()))
		}
	}
}

// claimName builds a safe single path element.
func claimName(sessionID, state, key string) string {
	clean := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
				return r
			}
			return '_'
		}, s)
	}
	return clean(sessionID) + "-" + clean(state) + "-" + clean(key)
}
