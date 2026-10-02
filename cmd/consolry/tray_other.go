//go:build !windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// waitForExit returns when Consolry is asked to stop. Off Windows there is no tray icon:
// it runs in a terminal, or as a service set up with -autostart on.
func waitForExit(ctx context.Context, _ context.CancelFunc, _, _ string) {
	<-ctx.Done()
}

const unit = `[Unit]
Description=Consolry game server panel
After=network-online.target
Wants=network-online.target

[Service]
ExecStart="%s" -no-browser -dir "%s"
Restart=on-failure
# Consolry saves and closes every server when asked to stop, so give it time.
KillSignal=SIGINT
TimeoutStopSec=90

[Install]
WantedBy=default.target
`

// setAutostart installs or removes a systemd service for the current user, so Consolry
// starts by itself and keeps running in the background. No root access is needed.
func setAutostart(on bool, home string) error {
	if runtime.GOOS != "linux" {
		return errors.New("starting automatically is only set up for Linux and Windows so far")
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(config, "systemd", "user", "consolry.service")
	systemctl := func(args ...string) error {
		out, err := exec.Command("systemctl", append([]string{"--user"}, args...)...).CombinedOutput()
		if err != nil {
			return fmt.Errorf("systemctl %v: %v: %s", args, err, out)
		}
		return nil
	}

	if !on {
		_ = systemctl("disable", "--now", "consolry.service")
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return systemctl("daemon-reload")
	}

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(fmt.Sprintf(unit, exe, home)), 0o644); err != nil {
		return err
	}
	if err := systemctl("daemon-reload"); err != nil {
		return err
	}
	if err := systemctl("enable", "--now", "consolry.service"); err != nil {
		return err
	}
	fmt.Println("Consolry is now running in the background and will start when you sign in.")
	fmt.Println("To keep it running while you are signed out, run once:  sudo loginctl enable-linger $USER")
	fmt.Println("See its log with:  journalctl --user -u consolry -f")
	return nil
}
