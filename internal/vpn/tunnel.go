package vpn

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"generic-arr-project/internal/closer"
	"generic-arr-project/internal/netx"

	"github.com/rs/zerolog"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

// Tunnel is a running userspace WireGuard interface. Its Dial/Listen
// methods create sockets inside the tunnel's network stack, so traffic
// on them can only leave through WireGuard. If the tunnel is down, those
// sockets simply cannot reach anything: there is no fallback path.
type Tunnel struct {
	cfg  *Config
	dev  *device.Device
	tnet *netstack.Net
	v4   netip.Addr // first IPv4 interface address, if any
	v6   netip.Addr // first IPv6 interface address, if any
}

// Up creates the tunnel and starts WireGuard. It does not wait for a
// handshake; use WaitForHandshake for that.
func Up(ctx context.Context, cfg *Config, log *zerolog.Logger) (*Tunnel, error) {
	uapi, err := cfg.uapi(ctx, net.DefaultResolver)
	if err != nil {
		return nil, err
	}

	addrs := make([]netip.Addr, 0, len(cfg.Addresses))
	t := &Tunnel{cfg: cfg}
	for _, p := range cfg.Addresses {
		a := p.Addr()
		addrs = append(addrs, a)
		if a.Is4() && !t.v4.IsValid() {
			t.v4 = a
		}
		if a.Is6() && !t.v6.IsValid() {
			t.v6 = a
		}
	}

	tunDev, tnet, err := netstack.CreateNetTUN(addrs, cfg.DNS, cfg.MTU)
	if err != nil {
		return nil, fmt.Errorf("create netstack tun: %w", err)
	}
	wgLog := log.With().Str("component", "wireguard").Logger()
	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), &device.Logger{
		Verbosef: func(format string, args ...any) { wgLog.Debug().Msgf(format, args...) },
		Errorf:   func(format string, args ...any) { wgLog.Error().Msgf(format, args...) },
	})
	if err := dev.IpcSet(uapi); err != nil {
		dev.Close()
		return nil, fmt.Errorf("configure wireguard device: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return nil, fmt.Errorf("bring up wireguard device: %w", err)
	}
	t.dev, t.tnet = dev, tnet
	return t, nil
}

// uapi renders the config in wireguard-go's UAPI format, resolving peer
// endpoint hostnames with resolver (the endpoint must be reachable outside
// the tunnel, so this lookup intentionally uses the host network).
func (c *Config) uapi(ctx context.Context, resolver *net.Resolver) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "private_key=%s\n", c.PrivateKey.Hex())
	fmt.Fprintf(&b, "listen_port=%d\n", c.ListenPort)
	b.WriteString("replace_peers=true\n")
	for i, p := range c.Peers {
		endpoint, err := resolveEndpoint(ctx, resolver, p.Endpoint)
		if err != nil {
			return "", fmt.Errorf("peer %d: %w", i, err)
		}
		fmt.Fprintf(&b, "public_key=%s\n", p.PublicKey.Hex())
		if p.PresharedKey != nil {
			fmt.Fprintf(&b, "preshared_key=%s\n", p.PresharedKey.Hex())
		}
		fmt.Fprintf(&b, "endpoint=%s\n", endpoint)
		fmt.Fprintf(&b, "persistent_keepalive_interval=%d\n", int(p.PersistentKeepalive.Seconds()))
		b.WriteString("replace_allowed_ips=true\n")
		for _, a := range p.AllowedIPs {
			fmt.Fprintf(&b, "allowed_ip=%s\n", a)
		}
	}
	return b.String(), nil
}

func resolveEndpoint(ctx context.Context, resolver *net.Resolver, endpoint string) (netip.AddrPort, error) {
	host, port, err := splitHostPort(endpoint)
	if err != nil {
		return netip.AddrPort{}, err
	}
	if a, err := netip.ParseAddr(host); err == nil {
		return netip.AddrPortFrom(a, port), nil
	}
	ips, err := resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("resolve endpoint %q: %w", host, err)
	}
	if len(ips) == 0 {
		return netip.AddrPort{}, fmt.Errorf("resolve endpoint %q: no addresses", host)
	}
	// Prefer IPv4: it works on more host networks.
	chosen := ips[0]
	for _, ip := range ips {
		if ip.Unmap().Is4() {
			chosen = ip.Unmap()
			break
		}
	}
	return netip.AddrPortFrom(chosen, port), nil
}

// Close shuts the tunnel down. Sockets created from it stop working.
func (t *Tunnel) Close() { t.dev.Close() }

// Name identifies this network in logs and status output.
func (t *Tunnel) Name() string { return "vpn" }

// HasIPv4 reports whether the tunnel carries IPv4.
func (t *Tunnel) HasIPv4() bool { return t.v4.IsValid() }

// HasIPv6 reports whether the tunnel carries IPv6.
func (t *Tunnel) HasIPv6() bool { return t.v6.IsValid() }

// IgnoredKeys returns wg-quick keys from the config that have no effect.
func (t *Tunnel) IgnoredKeys() []string { return t.cfg.IgnoredKeys }

// DialContext dials through the tunnel. Hostnames are resolved with the
// tunnel's DNS servers; without any configured, only IP literals work.
func (t *Tunnel) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	c, err := t.tnet.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("vpn dial %s %s: %w", network, address, err)
	}
	return c, nil
}

// LookupHost resolves host using the tunnel's DNS servers.
func (t *Tunnel) LookupHost(ctx context.Context, host string) ([]string, error) {
	if len(t.cfg.DNS) == 0 {
		return nil, fmt.Errorf("vpn lookup %q: tunnel has no DNS servers configured", host)
	}
	addrs, err := t.tnet.LookupContextHost(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("vpn lookup %q: %w", host, err)
	}
	return addrs, nil
}

// Listen opens a TCP listener on the tunnel interface.
func (t *Tunnel) Listen(network, address string) (net.Listener, error) {
	ap, err := t.localAddrPort(network, address)
	if err != nil {
		return nil, err
	}
	l, err := t.tnet.ListenTCPAddrPort(ap)
	if err != nil {
		return nil, fmt.Errorf("vpn listen %s %s: %w", network, ap, err)
	}
	return l, nil
}

// ListenPacket opens a UDP socket on the tunnel interface.
func (t *Tunnel) ListenPacket(network, address string) (net.PacketConn, error) {
	ap, err := t.localAddrPort(network, address)
	if err != nil {
		return nil, err
	}
	pc, err := t.tnet.ListenUDPAddrPort(ap)
	if err != nil {
		return nil, fmt.Errorf("vpn listen %s %s: %w", network, ap, err)
	}
	return pc, nil
}

// localAddrPort maps a listen address onto the tunnel's own interface
// address. An empty or unspecified host picks the tunnel address of the
// network's family ("tcp"/"udp" prefer IPv4).
func (t *Tunnel) localAddrPort(network, address string) (netip.AddrPort, error) {
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("vpn listen address %q: %w", address, err)
	}
	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return netip.AddrPort{}, fmt.Errorf("vpn listen address %q: invalid port", address)
	}

	var want netip.Addr
	switch {
	case strings.HasSuffix(network, "4"):
		want = t.v4
	case strings.HasSuffix(network, "6"):
		want = t.v6
	case t.v4.IsValid():
		want = t.v4
	default:
		want = t.v6
	}
	if host != "" {
		a, err := netip.ParseAddr(host)
		if err != nil {
			return netip.AddrPort{}, fmt.Errorf("vpn listen address %q: %w", address, err)
		}
		if !a.IsUnspecified() {
			want = a
		}
	}
	if !want.IsValid() {
		return netip.AddrPort{}, fmt.Errorf("vpn listen %s: tunnel has no address for this network", network)
	}
	return netip.AddrPortFrom(want, uint16(port)), nil
}

// PeerStatus is runtime state for one WireGuard peer.
type PeerStatus struct {
	PublicKeyHex  string
	Endpoint      string
	LastHandshake time.Time // zero if no handshake yet
	RxBytes       uint64
	TxBytes       uint64
}

// Status returns per-peer runtime state.
func (t *Tunnel) Status() ([]PeerStatus, error) {
	raw, err := t.dev.IpcGet()
	if err != nil {
		return nil, fmt.Errorf("read wireguard status: %w", err)
	}
	return parseStatus(raw)
}

func parseStatus(raw string) ([]PeerStatus, error) {
	var peers []PeerStatus
	var hsSec, hsNsec int64
	flush := func() {
		if len(peers) > 0 && hsSec > 0 {
			peers[len(peers)-1].LastHandshake = time.Unix(hsSec, hsNsec)
		}
		hsSec, hsNsec = 0, 0
	}
	for line := range strings.Lines(raw) {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		if key == "public_key" {
			flush()
			peers = append(peers, PeerStatus{PublicKeyHex: value})
			continue
		}
		if len(peers) == 0 {
			continue // interface-level keys
		}
		p := &peers[len(peers)-1]
		var err error
		switch key {
		case "endpoint":
			p.Endpoint = value
		case "last_handshake_time_sec":
			hsSec, err = strconv.ParseInt(value, 10, 64)
		case "last_handshake_time_nsec":
			hsNsec, err = strconv.ParseInt(value, 10, 64)
		case "rx_bytes":
			p.RxBytes, err = strconv.ParseUint(value, 10, 64)
		case "tx_bytes":
			p.TxBytes, err = strconv.ParseUint(value, 10, 64)
		}
		if err != nil {
			return nil, fmt.Errorf("parse wireguard status %s=%q: %w", key, value, err)
		}
	}
	flush()
	return peers, nil
}

// WaitForHandshake blocks until any peer completes a handshake. A handshake
// is only attempted once traffic is sent (or keepalive is configured), so
// this nudges the tunnel by dialing an address routed through the first peer.
func (t *Tunnel) WaitForHandshake(ctx context.Context) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	nudgeErr := t.nudge()
	for {
		peers, err := t.Status()
		if err != nil {
			return err
		}
		for _, p := range peers {
			if !p.LastHandshake.IsZero() {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			if nudgeErr != nil {
				return fmt.Errorf("wait for wireguard handshake: %w (sending the first packet failed: %w)", context.Cause(ctx), nudgeErr)
			}
			return fmt.Errorf("wait for wireguard handshake: %w", context.Cause(ctx))
		case <-ticker.C:
		}
	}
}

// nudge sends one UDP packet into the tunnel so WireGuard initiates a
// handshake. The destination is the first address of the first peer's
// AllowedIPs (for 0.0.0.0/0 that is 0.0.0.0, which we replace with a
// public resolver address that is routed through the tunnel).
// The error says why no packet was sent, which explains a handshake that
// never comes.
func (t *Tunnel) nudge() (err error) {
	dst, err := t.nudgeTarget()
	if err != nil {
		return err
	}
	c, err := t.tnet.DialUDPAddrPort(netip.AddrPort{}, dst)
	if err != nil {
		return err
	}
	defer closer.Join(&err, c)
	_, err = c.Write([]byte{0})
	return err
}

func (t *Tunnel) nudgeTarget() (netip.AddrPort, error) {
	for _, p := range t.cfg.Peers {
		for _, pfx := range p.AllowedIPs {
			a := pfx.Addr()
			if (a.Is4() && !t.v4.IsValid()) || (a.Is6() && !t.v6.IsValid()) {
				continue // tunnel has no source address of this family
			}
			if a.IsUnspecified() {
				// Default route: any address inside it is routed to the peer.
				if a.Is4() {
					a = netip.MustParseAddr("192.0.2.1") // TEST-NET-1, never answered
				} else {
					a = netip.MustParseAddr("2001:db8::1")
				}
			}
			return netip.AddrPortFrom(a, 9), nil // discard port
		}
	}
	return netip.AddrPort{}, errors.New("no routable AllowedIPs")
}

var _ netx.Network = (*Tunnel)(nil)

// ListenPort returns the local UDP port WireGuard is bound to.
func (t *Tunnel) ListenPort() (uint16, error) {
	raw, err := t.dev.IpcGet()
	if err != nil {
		return 0, fmt.Errorf("read wireguard status: %w", err)
	}
	for line := range strings.Lines(raw) {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "listen_port="); ok {
			port, err := strconv.ParseUint(v, 10, 16)
			if err != nil {
				return 0, fmt.Errorf("parse listen_port %q: %w", v, err)
			}
			return uint16(port), nil
		}
	}
	return 0, errors.New("wireguard status has no listen_port")
}

// Start brings a tunnel up and waits up to handshakeTimeout for a
// handshake, so a bad config or unreachable endpoint fails fast instead of
// leaving a tunnel that silently carries nothing.
func Start(ctx context.Context, cfg *Config, log *zerolog.Logger, handshakeTimeout time.Duration) (*Tunnel, error) {
	tun, err := Up(ctx, cfg, log)
	if err != nil {
		return nil, err
	}
	hsCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	if err := tun.WaitForHandshake(hsCtx); err != nil {
		tun.Close()
		return nil, fmt.Errorf("vpn: %w (check the endpoint and keys)", err)
	}
	return tun, nil
}
