//go:build !notray

package tray

import "testing"

func TestDesktopSession(t *testing.T) {
	cases := []struct {
		name    string
		env     map[string]string
		busFile bool
		want    bool
	}{
		{"headless", map[string]string{}, false, false},
		{"bus without display", map[string]string{"DBUS_SESSION_BUS_ADDRESS": "unix:path=/run/user/1000/bus"}, false, false},
		{"display without bus", map[string]string{"DISPLAY": ":0"}, false, false},
		{"x11", map[string]string{"DISPLAY": ":0", "DBUS_SESSION_BUS_ADDRESS": "unix:path=/run/user/1000/bus"}, false, true},
		{"wayland via runtime dir bus", map[string]string{"WAYLAND_DISPLAY": "wayland-0", "XDG_RUNTIME_DIR": "/run/user/1000"}, true, true},
		{"runtime dir without bus socket", map[string]string{"WAYLAND_DISPLAY": "wayland-0", "XDG_RUNTIME_DIR": "/run/user/1000"}, false, false},
	}
	for _, c := range cases {
		getenv := func(k string) string { return c.env[k] }
		exists := func(p string) bool { return c.busFile && p == "/run/user/1000/bus" }
		if got := desktopSession(getenv, exists); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
