// Package notify posts notifications when something new shows up in a view
// (a PR waiting on your review, a newly-assigned Linear issue). Channels:
// an in-app terminal toast, or an OS desktop notification, optionally with a
// sound.
package notify

import (
	"fmt"
	"os/exec"
	"runtime"

	tea "charm.land/bubbletea/v2"

	"github.com/sanity-labs/agenda/internal/ui"
)

// Notifier posts one notification and returns the message the UI should
// process (a ui.ToastMsg for the terminal channel, nil otherwise). Calls may
// block (they shell out), so run them from a tea.Cmd, not the update loop.
type Notifier interface {
	Notify(title, body string) tea.Msg
}

// New returns a notifier for the configured popup channel ("terminal" or
// "desktop"), or nil when the channel is off; callers treat a nil Notifier
// as "notifications disabled".
func New(popup string, sound bool) Notifier {
	if popup != "terminal" && popup != "desktop" {
		return nil
	}
	return system{desktop: popup == "desktop", sound: sound}
}

type system struct {
	desktop bool
	sound   bool
}

func (s system) Notify(title, body string) tea.Msg {
	if s.desktop {
		switch runtime.GOOS {
		case "darwin":
			script := fmt.Sprintf("display notification %q with title %q", body, title)
			_ = exec.Command("osascript", "-e", script).Run()
		default:
			_ = exec.Command("notify-send", title, body).Run()
		}
	}
	if s.sound {
		switch runtime.GOOS {
		case "darwin":
			_ = exec.Command("afplay", "/System/Library/Sounds/Ping.aiff").Run()
		default:
			_ = exec.Command("canberra-gtk-play", "-i", "message").Run()
		}
	}
	if !s.desktop {
		return ui.ToastMsg{Title: title, Body: body}
	}
	return nil
}
