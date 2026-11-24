package torrent

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// resolveUDPTrackers rewrites udp:// tracker hostnames to IP literals using
// the engine's network. anacrolix resolves UDP tracker hosts with the host
// resolver on every announce, which would reveal tracker names to the host's
// DNS server even when the packets themselves go through the VPN. HTTP(S)
// trackers need no rewrite: they are dialed by name through the network.
//
// Trackers that cannot be resolved are dropped and logged.
func (e *Engine) resolveUDPTrackers(tiers [][]string) [][]string {
	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()

	out := make([][]string, 0, len(tiers))
	for _, tier := range tiers {
		var kept []string
		for _, raw := range tier {
			rewritten, err := e.resolveUDPTracker(ctx, raw)
			if err != nil {
				e.log.Warn().Str("tracker", raw).Err(err).Msg("dropping tracker")
				continue
			}
			kept = append(kept, rewritten)
		}
		if len(kept) > 0 {
			out = append(out, kept)
		}
	}
	return out
}

func (e *Engine) resolveUDPTracker(ctx context.Context, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(u.Scheme, "udp") {
		return raw, nil
	}
	host := u.Hostname()
	if _, err := netip.ParseAddr(host); err == nil {
		return raw, nil
	}
	ips, err := e.net.LookupHost(ctx, host)
	if err != nil {
		return "", err
	}
	ip, ok := pickAddr(ips, e.net.HasIPv4(), e.net.HasIPv6())
	if !ok {
		return "", &net.AddrError{Err: "no address usable on this network", Addr: host}
	}
	u.Host = net.JoinHostPort(ip.String(), u.Port())
	return u.String(), nil
}

// pickAddr returns the first IPv4 address if v4 is usable, else the first
// IPv6 address if v6 is usable.
func pickAddr(ips []string, v4, v6 bool) (netip.Addr, bool) {
	var first6 netip.Addr
	for _, s := range ips {
		a, err := netip.ParseAddr(s)
		if err != nil {
			continue
		}
		a = a.Unmap()
		if a.Is4() && v4 {
			return a, true
		}
		if a.Is6() && v6 && !first6.IsValid() {
			first6 = a
		}
	}
	return first6, first6.IsValid()
}
