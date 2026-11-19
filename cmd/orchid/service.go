package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// install-service / uninstall-service register `orchid serve` to start on login:
// a launchd LaunchAgent on macOS, a systemd user service on Linux. The daemon
// reads its port/host from the saved config, so no flags are baked in.

const (
	macLabel    = "com.orchid.daemon"
	linuxUnit   = "orchid.service"
	servicePATH = "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin"
)

// selfExe returns the absolute path of the running orchid binary.
func selfExe() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return p, nil
}

func installServiceCmd(args []string) int {
	self, err := selfExe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid: cannot resolve own path:", err)
		return 1
	}
	switch runtime.GOOS {
	case "darwin":
		return installLaunchd(self)
	case "linux":
		return installSystemd(self)
	default:
		fmt.Fprintln(os.Stderr, "install-service: unsupported on", runtime.GOOS)
		return 1
	}
}

func uninstallServiceCmd(args []string) int {
	switch runtime.GOOS {
	case "darwin":
		return uninstallLaunchd()
	case "linux":
		return uninstallSystemd()
	default:
		fmt.Fprintln(os.Stderr, "uninstall-service: unsupported on", runtime.GOOS)
		return 1
	}
}

// ---- macOS (launchd LaunchAgent) ----

func macPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", macLabel+".plist"), nil
}

func installLaunchd(self string) int {
	plist, err := macPlistPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	home, _ := os.UserHomeDir()
	logPath := filepath.Join(home, "Library", "Logs", "orchid.log")
	if err := os.MkdirAll(filepath.Dir(plist), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>%s</string>
  <key>ProgramArguments</key><array><string>%s</string><string>serve</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>%s</string>
  <key>StandardErrorPath</key><string>%s</string>
  <key>EnvironmentVariables</key><dict><key>PATH</key><string>%s</string></dict>
</dict></plist>
`, macLabel, self, logPath, logPath, servicePATH)
	if err := os.WriteFile(plist, []byte(content), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "orchid: write plist:", err)
		return 1
	}
	// reload: unload an old copy (ignore error), then load.
	_ = exec.Command("launchctl", "unload", plist).Run()
	if out, err := exec.Command("launchctl", "load", "-w", plist).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "orchid: launchctl load: %v\n%s\n", err, out)
		return 1
	}
	fmt.Printf("Installed launchd service %s\n  plist: %s\n  logs:  %s\n  starts at login + now; stop with: orchid uninstall-service\n", macLabel, plist, logPath)
	return 0
}

func uninstallLaunchd() int {
	plist, err := macPlistPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	_ = exec.Command("launchctl", "unload", "-w", plist).Run()
	if err := os.Remove(plist); err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "orchid: remove plist:", err)
		return 1
	}
	fmt.Printf("Removed launchd service %s\n", macLabel)
	return 0
}

// ---- Linux (systemd user service) ----

func linuxUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", linuxUnit), nil
}

func installSystemd(self string) int {
	unit, err := linuxUnitPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(unit), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	content := fmt.Sprintf(`[Unit]
Description=orchid AI dev dashboard daemon
After=network.target

[Service]
ExecStart=%s serve
Restart=on-failure
RestartSec=2

[Install]
WantedBy=default.target
`, self)
	if err := os.WriteFile(unit, []byte(content), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "orchid: write unit:", err)
		return 1
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", linuxUnit).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "orchid: systemctl enable: %v\n%s\n", err, out)
		return 1
	}
	fmt.Printf("Installed systemd user service %s\n  unit: %s\n  logs: journalctl --user -u %s -f\n  starts at login + now; stop with: orchid uninstall-service\n", linuxUnit, unit, linuxUnit)
	return 0
}

func uninstallSystemd() int {
	unit, err := linuxUnitPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "orchid:", err)
		return 1
	}
	_ = exec.Command("systemctl", "--user", "disable", "--now", linuxUnit).Run()
	if err := os.Remove(unit); err != nil && !os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "orchid: remove unit:", err)
		return 1
	}
	_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
	fmt.Printf("Removed systemd user service %s\n", linuxUnit)
	return 0
}
