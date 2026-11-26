//go:build !windows

package console

// AttachParent reports whether os.Stderr can be written to. Outside
// Windows it always can.
func AttachParent() bool { return true }

// ShowError does nothing outside Windows: AttachParent always reports a
// usable stderr there, so errors are printed instead.
func ShowError(string) {}
