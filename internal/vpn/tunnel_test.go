package vpn_test

import (
	"context"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"generic-arr-project/internal/testutil"
	"generic-arr-project/internal/vpn/vpntest"
)

func TestTunnelTCPRoundTrip(t *testing.T) {
	a, b := vpntest.Pair(t)

	l, err := b.Listen("tcp4", ":8080")
	if err != nil {
		t.Fatal(err)
	}
	defer testutil.Close(t, l)
	remote := make(chan net.Addr, 1)
	served := make(chan struct{})
	// Runs after the deferred closes below, which end the echo loop, so the
	// server goroutine finishes before the test does.
	t.Cleanup(func() { <-served })
	go func() {
		defer close(served)
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer testutil.Close(t, c)
		remote <- c.RemoteAddr()
		_, _ = io.Copy(c, c)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := a.DialContext(ctx, "tcp4", netip.AddrPortFrom(vpntest.AddrB, 8080).String())
	if err != nil {
		t.Fatalf("dial through tunnel: %v", err)
	}
	defer testutil.Close(t, c)
	if _, err := c.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("echo = %q, %v", buf, err)
	}

	// The server must see the tunnel address, not a host address.
	got, err := netip.ParseAddrPort((<-remote).String())
	if err != nil {
		t.Fatal(err)
	}
	if got.Addr() != vpntest.AddrA {
		t.Errorf("server saw %v, want tunnel address %v", got.Addr(), vpntest.AddrA)
	}

	peers, err := a.Status()
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].LastHandshake.IsZero() || peers[0].TxBytes == 0 {
		t.Errorf("status = %+v, want one peer with a handshake and traffic", peers)
	}
}

// Tunnel sockets must never reach the host network, even for addresses the
// host could reach directly.
func TestTunnelCannotReachHostNetwork(t *testing.T) {
	a, _ := vpntest.Pair(t)

	hostListener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer testutil.Close(t, hostListener)
	accepted := make(chan struct{}, 1)
	go func() {
		if c, err := hostListener.Accept(); err == nil {
			testutil.Close(t, c)
			accepted <- struct{}{}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if c, err := a.DialContext(ctx, "tcp4", hostListener.Addr().String()); err == nil {
		testutil.Close(t, c)
		t.Fatal("tunnel dial to a host-only address succeeded")
	}
	select {
	case <-accepted:
		t.Fatal("host listener received a connection from the tunnel")
	case <-time.After(200 * time.Millisecond):
	}
}

// When the remote end goes away, dials fail instead of falling back.
func TestTunnelDownFailsClosed(t *testing.T) {
	a, b := vpntest.Pair(t)
	l, err := b.Listen("tcp4", ":8081")
	if err != nil {
		t.Fatal(err)
	}
	testutil.Close(t, l)
	b.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if c, err := a.DialContext(ctx, "tcp4", netip.AddrPortFrom(vpntest.AddrB, 8081).String()); err == nil {
		testutil.Close(t, c)
		t.Fatal("dial succeeded with the remote tunnel down")
	}
}

func TestTunnelLookupWithoutDNSFails(t *testing.T) {
	a, _ := vpntest.Pair(t)
	if _, err := a.LookupHost(context.Background(), "example.com"); err == nil {
		t.Fatal("lookup succeeded on a tunnel without DNS servers")
	}
}
