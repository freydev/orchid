//go:build !darwin

package notify

// Stubs so the cross-platform Notify/Prime dispatch compiles off macOS. The
// macOS notification helper (notify_darwin.go) is darwin-only.
func macNotify(title, body, url string) {}
func macPrime()                          {}
