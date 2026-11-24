// Package lan announces arrd on the local network over multicast DNS, so
// other devices can reach the web UI as http://arrd.local:PORT.
//
// Its sockets are plain host sockets: LAN discovery must never go through
// the download VPN.
package lan

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"github.com/pion/mdns/v2"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// Hostname is the name arrd answers for over mDNS.
const Hostname = "arrd.local"

// mdnsPort is the well-known mDNS port (RFC 6762).
const mdnsPort = "5353"

// Responder answers mDNS queries for a hostname with this machine's LAN
// addresses. It shares port 5353 with any system responder (Avahi,
// Bonjour, Windows' own mDNS).
type Responder struct {
	conn *mdns.Conn
}

// ResponderOptions configure a Responder.
type ResponderOptions struct {
	Hostname string
	Log      *slog.Logger
	// IncludeLoopback also answers on loopback interfaces; for tests.
	IncludeLoopback bool
}

// StartResponder starts answering queries for opts.Hostname. It listens on
// IPv4 and, where available, IPv6; it fails only if neither works.
func StartResponder(opts ResponderOptions) (*Responder, error) {
	if opts.Hostname == "" || opts.Log == nil {
		return nil, errors.New("lan: Hostname and Log are required")
	}
	lc := net.ListenConfig{Control: reuseAddr}
	ctx := context.Background()

	var pc4 *ipv4.PacketConn
	l4, err4 := lc.ListenPacket(ctx, "udp4", net.JoinHostPort("0.0.0.0", mdnsPort))
	if err4 == nil {
		pc4 = ipv4.NewPacketConn(l4)
	}
	var pc6 *ipv6.PacketConn
	l6, err6 := lc.ListenPacket(ctx, "udp6", net.JoinHostPort("::", mdnsPort))
	if err6 == nil {
		pc6 = ipv6.NewPacketConn(l6)
	}
	if pc4 == nil && pc6 == nil {
		return nil, fmt.Errorf("listen for mDNS on port %s: %w", mdnsPort, errors.Join(err4, err6))
	}
	if err6 != nil {
		opts.Log.Debug("mDNS over IPv6 unavailable", "err", err6)
	}

	conn, err := mdns.Server(pc4, pc6, &mdns.Config{
		Name:            "arrd",
		LocalNames:      []string{opts.Hostname},
		LoggerFactory:   slogFactory{opts.Log},
		IncludeLoopback: opts.IncludeLoopback,
	})
	if err != nil {
		closeAll(pc4, pc6)
		return nil, fmt.Errorf("start mDNS responder: %w", err)
	}
	opts.Log.Info("announcing on the local network over mDNS", "hostname", opts.Hostname)
	return &Responder{conn: conn}, nil
}

// Close stops answering queries.
func (r *Responder) Close() error {
	if err := r.conn.Close(); err != nil {
		return fmt.Errorf("stop mDNS responder: %w", err)
	}
	return nil
}

func closeAll(pc4 *ipv4.PacketConn, pc6 *ipv6.PacketConn) {
	if pc4 != nil {
		_ = pc4.Close()
	}
	if pc6 != nil {
		_ = pc6.Close()
	}
}
