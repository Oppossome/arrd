// Package netx defines the single seam through which arrd subsystems create
// sockets. A subsystem is handed exactly one Network and must not touch the
// net package directly, so routing it through a VPN is a wiring decision
// rather than something each subsystem has to get right.
package netx

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

// Network creates connections, listeners and DNS lookups on one routing
// domain (the host network, or a VPN tunnel).
type Network interface {
	// Name identifies the network in logs ("direct", "vpn").
	Name() string
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
	// Listen opens a stream listener ("tcp", "tcp4", "tcp6").
	Listen(network, address string) (net.Listener, error)
	// ListenPacket opens a datagram socket ("udp", "udp4", "udp6").
	ListenPacket(network, address string) (net.PacketConn, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
	HasIPv4() bool
	HasIPv6() bool
}

// HTTPClient returns an HTTP client whose connections all use n. Proxy
// environment variables are ignored on purpose: a proxy reached outside n
// would bypass it.
func HTTPClient(n Network, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:               nil,
			DialContext:         n.DialContext,
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}
}

// Direct is the host's own network stack.
type Direct struct{}

var _ Network = Direct{}

func (Direct) Name() string { return "direct" }

func (Direct) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, network, address)
}

func (Direct) Listen(network, address string) (net.Listener, error) {
	return net.Listen(network, address)
}

func (Direct) ListenPacket(network, address string) (net.PacketConn, error) {
	return net.ListenPacket(network, address)
}

func (Direct) LookupHost(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

// The host may or may not have working IPv4/IPv6; report both and let
// dials fail naturally, as any normal program would.
func (Direct) HasIPv4() bool { return true }
func (Direct) HasIPv6() bool { return true }

// PublicIP asks an IP echo service that returns the caller's address as
// plain text (e.g. https://api.ipify.org) which address traffic on n
// appears to come from.
func PublicIP(ctx context.Context, n Network, echoURL string) (netip.Addr, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, echoURL, nil)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("public ip: %w", err)
	}
	resp, err := HTTPClient(n, 20*time.Second).Do(req)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("public ip via %s: %w", n.Name(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return netip.Addr{}, fmt.Errorf("public ip via %s: %s returned %s", n.Name(), echoURL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 256))
	if err != nil {
		return netip.Addr{}, fmt.Errorf("public ip via %s: read body: %w", n.Name(), err)
	}
	ip, err := netip.ParseAddr(strings.TrimSpace(string(body)))
	if err != nil {
		return netip.Addr{}, fmt.Errorf("public ip via %s: unexpected response %q", n.Name(), body)
	}
	return ip, nil
}
