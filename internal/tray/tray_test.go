package tray

import "testing"

func TestBrowseURL(t *testing.T) {
	cases := map[string]string{
		"127.0.0.1:8484":  "http://127.0.0.1:8484",
		":8484":           "http://127.0.0.1:8484",
		"0.0.0.0:8484":    "http://127.0.0.1:8484",
		"[::]:8484":       "http://[::1]:8484",
		"example.test:80": "http://example.test:80",
	}
	for listen, want := range cases {
		if got := BrowseURL(listen); got != want {
			t.Errorf("BrowseURL(%q) = %q, want %q", listen, got, want)
		}
	}
}
