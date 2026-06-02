// Package notify sends best-effort OS desktop notifications. It is used to alert
// the user when an agent finishes working. Failures are ignored: a missing
// notifier or unsupported platform is a no-op.
package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Notify shows a desktop notification with the given title and body. It returns
// immediately (the notifier runs detached) and never blocks the caller.
func Notify(title, body string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf("display notification %q with title %q", body, title)
		cmd = exec.Command("osascript", "-e", script)
	case "linux":
		if _, err := exec.LookPath("notify-send"); err != nil {
			return
		}
		cmd = exec.Command("notify-send", title, body)
	default:
		return
	}
	_ = cmd.Start()
	if cmd.Process != nil {
		go cmd.Wait() // reap without blocking
	}
}
