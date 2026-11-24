//go:build (linux || windows) && !notray

package tray

import "os/exec"

// startDetached starts cmd and reaps it in the background.
func startDetached(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait() //nolint:errcheck // the browser launcher's exit status is irrelevant
	return nil
}
