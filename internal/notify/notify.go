// Package notify sends best-effort OS desktop notifications. It is used to alert
// the user when an agent finishes working. Failures are ignored: a missing
// notifier or unsupported platform is a no-op.
//
// On macOS, a bare `osascript "display notification"` is attributed to "Script
// Editor" and recent macOS silently drops those from background daemons. So we
// ship a tiny signed helper app (see macapp/) that registers as its own
// "Orchid" notification source; see notify_darwin.go.
package notify

import (
	"os/exec"
	"runtime"
)

// Notify shows a desktop notification with the given title and body. url, when
// non-empty, is a deep link opened when the user clicks the notification
// (macOS only; ignored where the platform notifier can't carry an action). It
// returns immediately (the notifier runs detached) and never blocks the caller.
func Notify(title, body, url string) {
	switch runtime.GOOS {
	case "darwin":
		macNotify(title, body, url)
	case "linux":
		if _, err := exec.LookPath("notify-send"); err != nil {
			return
		}
		runDetached(exec.Command("notify-send", title, body))
	}
}

// Prime sets up the notification backend up front (no-op off macOS). Calling it
// at startup moves the one-time helper-app install — and the first-launch
// notification-permission grant — off the first real alert.
func Prime() {
	if runtime.GOOS == "darwin" {
		macPrime()
	}
}

// runDetached starts cmd without blocking and reaps it in the background.
func runDetached(cmd *exec.Cmd) {
	if err := cmd.Start(); err != nil {
		return
	}
	if cmd.Process != nil {
		go cmd.Wait()
	}
}
