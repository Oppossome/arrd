package vpn

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

const (
	testPriv = "yAnz5TF+lXXJte14tji3zlMNq+hd2rYUIgJBgB3fBmk="
	testPub  = "xTIBA5rboUvnH4htodjb6e697QjLERt1NAB4mZqp8Dg="
	testPSK  = "AAECAwQFBgcICQoLDA0ODxAREhMUFRYXGBkaGxwdHh8="
)

const providerConf = `
# Typical provider-issued config
[Interface]
PrivateKey = ` + testPriv + `
Address = 10.64.1.2/32, fc00:bbbb::2/128
DNS = 10.64.0.1, corp.example
MTU = 1380
PostUp = iptables -A ...

[Peer]
PublicKey = ` + testPub + `
PresharedKey = ` + testPSK + `
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = 198.51.100.7:51820
PersistentKeepalive = 25
`

func TestParseConfig(t *testing.T) {
	cfg, err := ParseConfig(strings.NewReader(providerConf))
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if len(cfg.Addresses) != 2 || !cfg.HasIPv4() || !cfg.HasIPv6() {
		t.Errorf("Addresses = %v", cfg.Addresses)
	}
	if len(cfg.DNS) != 1 || cfg.DNS[0] != netip.MustParseAddr("10.64.0.1") {
		t.Errorf("DNS = %v", cfg.DNS)
	}
	if cfg.MTU != 1380 {
		t.Errorf("MTU = %d", cfg.MTU)
	}
	if len(cfg.IgnoredKeys) != 2 {
		t.Errorf("IgnoredKeys = %v, want PostUp and the DNS search domain", cfg.IgnoredKeys)
	}
	p := cfg.Peers[0]
	if p.PresharedKey == nil || p.PersistentKeepalive != 25*time.Second || len(p.AllowedIPs) != 2 {
		t.Errorf("peer = %+v", p)
	}

	uapi, err := cfg.uapi(context.Background(), net.DefaultResolver)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"private_key=c809f3e5317e9575c9b5ed78b638b7ce530dabe85ddab6142202418" + "01ddf0669",
		"endpoint=198.51.100.7:51820",
		"persistent_keepalive_interval=25",
		"allowed_ip=0.0.0.0/0",
		"allowed_ip=::/0",
	} {
		if !strings.Contains(uapi, want+"\n") {
			t.Errorf("uapi missing %q:\n%s", want, uapi)
		}
	}
}

func TestParseConfigErrors(t *testing.T) {
	peer := "[Peer]\nPublicKey = " + testPub + "\nAllowedIPs = 0.0.0.0/0\nEndpoint = 1.2.3.4:51820\n"
	iface := "[Interface]\nPrivateKey = " + testPriv + "\nAddress = 10.0.0.2/32\n"
	tests := []struct {
		name, conf, wantErr string
	}{
		{"no private key", "[Interface]\nAddress = 10.0.0.2/32\n" + peer, "PrivateKey is required"},
		{"no address", "[Interface]\nPrivateKey = " + testPriv + "\n" + peer, "Address is required"},
		{"no peer", iface, "at least one [Peer]"},
		{"bad key", "[Interface]\nPrivateKey = not*base64\n", "decode key"},
		{"unknown key", iface + "Bogus = 1\n" + peer, "unknown [Interface] key"},
		{"endpoint without port", iface + strings.Replace(peer, "1.2.3.4:51820", "1.2.3.4", 1), "missing port"},
		{"unknown section", "[Wat]\n", "unknown section"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig(strings.NewReader(tt.conf))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	raw := "private_key=abc\nlisten_port=51820\npublic_key=p1\nendpoint=1.2.3.4:51820\nlast_handshake_time_sec=1700000000\nlast_handshake_time_nsec=5\nrx_bytes=10\ntx_bytes=20\npublic_key=p2\nlast_handshake_time_sec=0\nlast_handshake_time_nsec=0\n"
	peers, err := parseStatus(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 2 {
		t.Fatalf("got %d peers", len(peers))
	}
	if !peers[0].LastHandshake.Equal(time.Unix(1700000000, 5)) || peers[0].RxBytes != 10 || peers[0].TxBytes != 20 {
		t.Errorf("peer 0 = %+v", peers[0])
	}
	if !peers[1].LastHandshake.IsZero() {
		t.Errorf("peer 1 handshake = %v, want zero", peers[1].LastHandshake)
	}
}
