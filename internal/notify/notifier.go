package notify

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"time"
)

// Event is one transition ready to announce.
type Event struct {
	SessionID string
	State     string // StateWaiting / StateFinished / StateError
	// Key identifies this transition across fleet instances; see TransitionKey.
	Key                      string
	Title, Subtitle, Message string
	Sound                    bool
}

// Notifier dedups an Event across instances and hands it to a Deliverer.
type Notifier struct {
	Dedup   *Dedup
	Deliver Deliverer
}

// New returns a Notifier using the real claim directory and the OS.
func New() *Notifier { return &Notifier{Dedup: NewDedup(DefaultDir()), Deliver: SystemDeliverer{}} }

// Send claims ev and delivers it. sent is false when another instance already
// claimed it (not an error). The caller runs this off the UI goroutine.
func (n *Notifier) Send(ctx context.Context, ev Event) (sent bool, mech string, err error) {
	won, err := n.Dedup.Claim(ev.SessionID, ev.State, ev.Key)
	if err != nil || !won {
		return false, "", err
	}
	mech, err = n.Deliver.Deliver(ctx, Notification{
		Title:    Title(ev.State, ev.Title),
		Subtitle: ev.Subtitle,
		Message:  ev.Message,
		Group:    "fleet-" + ev.SessionID,
		Sound:    SoundFor(ev.State, ev.Sound),
	})
	return err == nil, mech, err
}

// TransitionKey is the part of a dedup claim that is the same in every
// instance watching the transition and different for the next one.
//
// Chosen: the timestamp (unix seconds) of the latest hook event for the
// session, read from the shared hook status file, so every instance sees the
// same value. When a hook caused the state (hookStatus == state; every Claude
// waiting/finished/error) that is the whole key. When the pane caused it
// (Codex, or a prompt no hook announced) the hook is still the previous one, so
// a short hash of the message is added to tell two prompts apart.
func TransitionKey(state, hookStatus string, hookAt time.Time, message string) string {
	key := "h" + strconv.FormatInt(hookAt.Unix(), 10)
	if hookStatus == state || state == StateError {
		return key
	}
	sum := sha1.Sum([]byte(message))
	return key + "-" + hex.EncodeToString(sum[:4])
}
