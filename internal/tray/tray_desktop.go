//go:build (linux || windows) && !notray

package tray

import (
	"context"
	"os"
	"runtime"

	"fyne.io/systray"
)

// Run shows the tray icon until ctx is done. It returns immediately when no
// desktop session is available.
func Run(ctx context.Context, o Options) {
	if !desktopSession(os.Getenv, fileExists) {
		o.Log.Info().Msg("tray: no desktop session; tray icon disabled")
		return
	}
	// The native event loop must stay on one OS thread (required on Windows).
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	systray.Run(func() { onReady(ctx, o) }, nil)
}

func onReady(ctx context.Context, o Options) {
	systray.SetIcon(trayIcon())
	systray.SetTitle("arrd")
	systray.SetTooltip("arrd")
	open := systray.AddMenuItem("Open arrd", "Open the arrd web interface")
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "Stop arrd")
	go func() {
		defer systray.Quit()
		for {
			select {
			case <-ctx.Done():
				return
			case <-open.ClickedCh:
				if err := openBrowser(o.URL); err != nil {
					o.Log.Error().Str("url", o.URL).Err(err).Msg("tray: open browser")
				}
			case <-quit.ClickedCh:
				o.Log.Info().Msg("tray: quit requested")
				o.Quit()
				return
			}
		}
	}()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
