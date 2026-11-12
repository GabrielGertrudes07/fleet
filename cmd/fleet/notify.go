package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/brizzai/fleet/internal/config"
	"github.com/brizzai/fleet/internal/notify"
)

// runNotify handles `fleet notify test`: one sample notification through the
// same delivery path the TUI uses, so "why don't I get notifications?" has an
// answer that does not involve waiting for a session to stop.
func runNotify(args []string) {
	if len(args) != 1 || args[0] != "test" {
		fmt.Fprintln(os.Stderr, "Usage: fleet notify test")
		os.Exit(1)
	}
	cfg := config.Load()
	n := notify.Notification{
		Title:    notify.Title(notify.StateWaiting, "Sessão de exemplo"),
		Subtitle: notify.Subtitle("", "", "fleet", "main"),
		Message:  "Permissão: Bash(git push origin main)",
		Group:    "fleet-test",
		Sound:    notify.SoundFor(notify.StateWaiting, cfg.IsNotifySound()),
	}
	ctx, cancel := context.WithTimeout(context.Background(), notify.Timeout)
	defer cancel()
	mech, err := notify.SystemDeliverer{}.Deliver(ctx, n)
	if errors.Is(err, notify.ErrUnsupported) {
		fatalf("No notification mechanism available (macOS: terminal-notifier or osascript; Linux: notify-send).")
	}
	if err != nil {
		fatalf("Notification via %s failed: %v", mech, err)
	}
	fmt.Printf("Sent a test notification via %s.\n", mech)
	if !cfg.IsNotificationsEnabled() {
		fmt.Println(`Notifications are currently off: set "notifications": {"enabled": true} in config.json (or use Settings) to get them for real sessions.`)
	}
}
