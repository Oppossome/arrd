package torrent

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"generic-arr-project/internal/netx"
)

// directNoV6 is the host network restricted to IPv4, so tests don't depend
// on the host having working IPv6.
type directNoV6 struct{ netx.Direct }

func (directNoV6) HasIPv6() bool { return false }

// fakeDNS resolves from a fixed table and records every lookup.
type fakeDNS struct {
	directNoV6
	table   map[string][]string
	lookups []string
}

func (f *fakeDNS) Name() string { return "fake" }

func (f *fakeDNS) LookupHost(_ context.Context, host string) ([]string, error) {
	f.lookups = append(f.lookups, host)
	if ips, ok := f.table[host]; ok {
		return ips, nil
	}
	return nil, errors.New("no such host")
}

func TestResolveUDPTrackers(t *testing.T) {
	dns := &fakeDNS{table: map[string][]string{
		"tracker.example": {"2001:db8::1", "203.0.113.5"},
		"v6only.example":  {"2001:db8::2"},
	}}
	e := &Engine{net: dns, log: quietLog()}

	got := e.resolveUDPTrackers([][]string{
		{"udp://tracker.example:1337/announce", "https://http.example/announce"},
		{"udp://v6only.example:80", "udp://missing.example:80"},
		{"udp://198.51.100.1:6969/announce"},
	})
	want := [][]string{
		{"udp://203.0.113.5:1337/announce", "https://http.example/announce"},
		{"udp://198.51.100.1:6969/announce"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
	// HTTP trackers and IP literals must not trigger lookups.
	if wantLookups := []string{"tracker.example", "v6only.example", "missing.example"}; !reflect.DeepEqual(dns.lookups, wantLookups) {
		t.Errorf("lookups = %v, want %v", dns.lookups, wantLookups)
	}
}
