// Package vpntest builds pairs of in-process WireGuard tunnels connected
// over loopback UDP, for tests that need a real tunnel without a VPN provider.
package vpntest

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"generic-arr-project/internal/vpn"
)

// Tunnel addresses used by Pair.
var (
	AddrA = netip.MustParseAddr("10.99.0.1")
	AddrB = netip.MustParseAddr("10.99.0.2")
)

// Pair starts two tunnels, A (10.99.0.1) and B (10.99.0.2), that route
// 10.99.0.0/24 to each other, and waits for their handshake. Both are
// closed when the test ends.
func Pair(t testing.TB) (a, b *vpn.Tunnel) {
	t.Helper()
	privA, pubA := keyPair(t)
	privB, pubB := keyPair(t)

	cfgA := &vpn.Config{
		PrivateKey: privA,
		Addresses:  []netip.Prefix{netip.PrefixFrom(AddrA, 32)},
		MTU:        vpn.DefaultMTU,
		Peers: []vpn.PeerConfig{{
			PublicKey:  pubB,
			Endpoint:   "127.0.0.1:1", // replaced once B is up
			AllowedIPs: []netip.Prefix{netip.MustParsePrefix("10.99.0.0/24")},
		}},
	}
	cfgB := &vpn.Config{
		PrivateKey: privB,
		Addresses:  []netip.Prefix{netip.PrefixFrom(AddrB, 32)},
		MTU:        vpn.DefaultMTU,
		Peers: []vpn.PeerConfig{{
			PublicKey: pubA,
			// B never initiates; it learns A's address from A's first packet.
			Endpoint:   "127.0.0.1:1",
			AllowedIPs: []netip.Prefix{netip.MustParsePrefix("10.99.0.0/24")},
		}},
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	if b, err = vpn.Up(ctx, cfgB, log); err != nil {
		t.Fatalf("tunnel B: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	// Both tunnels bind a port the OS picks, so parallel tests never collide.
	portB, err := b.ListenPort()
	if err != nil {
		t.Fatal(err)
	}
	cfgA.Peers[0].Endpoint = fmt.Sprintf("127.0.0.1:%d", portB)
	if a, err = vpn.Up(ctx, cfgA, log); err != nil {
		t.Fatalf("tunnel A: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	if err := a.WaitForHandshake(ctx); err != nil {
		t.Fatalf("handshake: %v", err)
	}
	return a, b
}

func keyPair(t testing.TB) (priv, pub vpn.Key) {
	t.Helper()
	priv, pub, err := vpn.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return priv, pub
}

// Server starts one tunnel (at AddrB) that accepts a client, and returns
// it with a wg-quick config text for that client (at AddrA). The client
// side is not started; feed the text to whatever is under test.
func Server(t testing.TB) (server *vpn.Tunnel, clientConfig string) {
	t.Helper()
	privClient, pubClient := keyPair(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server, pubServer, port, err := NewServer(ctx, pubClient)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	clientConfig = fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32

[Peer]
PublicKey = %s
AllowedIPs = 10.99.0.0/24
Endpoint = 127.0.0.1:%d
`, base64.StdEncoding.EncodeToString(privClient[:]), AddrA, base64.StdEncoding.EncodeToString(pubServer[:]), port)
	return server, clientConfig
}

// NewServer starts a tunnel at AddrB that accepts the client with public
// key clientPub at AddrA, as a VPN provider's server does once a client
// key is registered. It returns the server's public key and the loopback
// UDP port it listens on. The caller closes the server. Unlike Server it
// does not need a testing.TB, so fake provider handlers can call it.
func NewServer(ctx context.Context, clientPub vpn.Key) (server *vpn.Tunnel, serverPub vpn.Key, port uint16, err error) {
	privServer, pubServer, err := vpn.GenerateKeyPair()
	if err != nil {
		return nil, vpn.Key{}, 0, err
	}
	cfg := &vpn.Config{
		PrivateKey: privServer,
		Addresses:  []netip.Prefix{netip.PrefixFrom(AddrB, 32)},
		MTU:        vpn.DefaultMTU,
		Peers: []vpn.PeerConfig{{
			PublicKey:  clientPub,
			Endpoint:   "127.0.0.1:1", // learned from the client's first packet
			AllowedIPs: []netip.Prefix{netip.PrefixFrom(AddrA, 32)},
		}},
	}
	server, err = vpn.Up(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		return nil, vpn.Key{}, 0, fmt.Errorf("vpn server: %w", err)
	}
	if port, err = server.ListenPort(); err != nil {
		server.Close()
		return nil, vpn.Key{}, 0, err
	}
	return server, pubServer, port, nil
}
