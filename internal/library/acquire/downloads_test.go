package acquire

import (
	"net/netip"
	"slices"
	"testing"

	"generic-arr-project/internal/torrent"
)

func TestCountPeerCountries(t *testing.T) {
	peer := func(ip, country string) Peer {
		return Peer{Peer: torrent.Peer{Addr: netip.MustParseAddr(ip)}, Country: country}
	}
	got := countPeerCountries([]Peer{
		peer("198.51.100.1", "DE"),
		peer("198.51.100.2", "NZ"),
		peer("198.51.100.3", "DE"),
		peer("10.0.0.1", ""),
		peer("198.51.100.4", "AT"),
	})
	want := []CountryCount{{"DE", 2}, {"", 1}, {"AT", 1}, {"NZ", 1}}
	if !slices.Equal(got, want) {
		t.Errorf("countPeerCountries = %v, want %v", got, want)
	}
	if got := countPeerCountries(nil); len(got) != 0 {
		t.Errorf("no peers = %v", got)
	}
}
