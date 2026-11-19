//go:build darwin

package notify

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// macAppFS holds the prebuilt, ad-hoc-signed Orchid.app helper. `all:` is
// required so the signature dir (Contents/_CodeSignature, leading underscore)
// is embedded — go:embed skips `_`/`.` names otherwise. The bytes are written
// to disk verbatim so the code signature stays valid.
//
//go:embed all:macapp/Orchid.app
var macAppFS embed.FS

const (
	macBundleSrc  = "macapp/Orchid.app"
	macAppVersion = "5" // bump when macapp/ changes to force a re-extract
)

var (
	macOnce sync.Once
	macBin  string // extracted orchid-notify path; "" => fall back to osascript
)

func macPrime() { macOnce.Do(setupMacApp) }

func macNotify(title, body, url string) {
	macOnce.Do(setupMacApp)
	if macBin == "" {
		// Fallback: the helper app is unavailable (extract failed). Post via the
		// legacy osascript path so notifications still work, just unbranded (and
		// without a clickable deep link).
		script := fmt.Sprintf("display notification %q with title %q", body, title)
		runDetached(exec.Command("osascript", "-e", script))
		return
	}
	// The helper is a real signed app: the notification is attributed to the
	// "Orchid" source, and clicking it opens url (the focused session).
	runDetached(exec.Command(macBin, title, body, url))
}

// setupMacApp installs the helper app under ~/Library/Application Support/orchid
// (extracting it verbatim the first time / after a version bump) and registers
// it with macOS so notifications are attributed to "Orchid". Runs at most once.
func setupMacApp() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	base := filepath.Join(home, "Library", "Application Support", "orchid")
	app := filepath.Join(base, "Orchid.app")
	bin := filepath.Join(app, "Contents", "MacOS", "orchid-notify")
	marker := filepath.Join(base, ".notify-version")

	if data, _ := os.ReadFile(marker); string(data) != macAppVersion {
		if err := extractMacApp(app); err != nil {
			return
		}
		_ = os.WriteFile(marker, []byte(macAppVersion), 0o644)
		registerMacApp(app)
	}
	if _, err := os.Stat(bin); err != nil {
		return
	}
	macBin = bin
}

// extractMacApp writes the embedded bundle to dest, byte-for-byte (preserving
// the code signature) with the executable marked +x.
func extractMacApp(dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	exe := filepath.Join("Contents", "MacOS", "orchid-notify")
	return fs.WalkDir(macAppFS, macBundleSrc, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(macBundleSrc, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(out, 0o755)
		}
		data, err := macAppFS.ReadFile(p)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if rel == exe {
			mode = 0o755
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, data, mode)
	})
}

// registerMacApp tells LaunchServices about the bundle and launches it once so
// macOS registers the notification provider and grants delivery authorization.
func registerMacApp(app string) {
	const lsregister = "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
	_ = exec.Command(lsregister, "-f", app).Run()
	_ = exec.Command("open", app).Run() // no args => setup-only launch, posts nothing
}
