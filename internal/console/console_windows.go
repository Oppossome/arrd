//go:build windows

package console

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// attachParentProcess is ATTACH_PARENT_PROCESS, (DWORD)-1.
const attachParentProcess = ^uint32(0)

var procAttachConsole = windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")

// AttachParent makes os.Stdout and os.Stderr usable when the process was
// started without them, then reports whether os.Stderr can be written to.
//
// Streams that were redirected (arrd > log.txt) are kept. Missing ones are
// pointed at the console of the parent process, if it has one (arrd run from
// cmd or PowerShell). If the program was started from Explorer or the tray,
// nothing is attached and it returns false.
func AttachParent() bool {
	stdoutOK, stderrOK := validStdHandle(windows.STD_OUTPUT_HANDLE), validStdHandle(windows.STD_ERROR_HANDLE)
	if stdoutOK && stderrOK {
		return true
	}
	if r, _, _ := procAttachConsole.Call(uintptr(attachParentProcess)); r == 0 {
		return stderrOK
	}
	conout, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return stderrOK
	}
	if !stdoutOK {
		os.Stdout = conout
		_ = windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, windows.Handle(conout.Fd())) // best effort, for child processes
	}
	if !stderrOK {
		os.Stderr = conout
		_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(conout.Fd()))
	}
	return true
}

func validStdHandle(which uint32) bool {
	h, err := windows.GetStdHandle(which)
	return err == nil && h != 0 && h != windows.InvalidHandle
}

// ShowError shows msg in a modal error dialog, for failures that would
// otherwise vanish because there is no console to print them to.
func ShowError(msg string) {
	text := windows.StringToUTF16Ptr(strings.ReplaceAll(msg, "\x00", ""))
	caption := windows.StringToUTF16Ptr("arrd")
	// Nothing is left to report a failure to: this is the last resort.
	_, _ = windows.MessageBox(0, text, caption, windows.MB_OK|windows.MB_ICONERROR)
}
