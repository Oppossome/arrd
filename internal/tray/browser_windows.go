//go:build !notray

package tray

import "os/exec"

func openBrowser(url string) error {
	return startDetached(exec.Command("rundll32", "url.dll,FileProtocolHandler", url))
}
