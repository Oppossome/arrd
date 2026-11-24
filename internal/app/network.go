package app

import (
	"context"

	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/torrent"
	"generic-arr-project/internal/vpn/provider"

	"github.com/rs/zerolog"
)

// newRuntimeFactory builds the torrent engine, on the VPN tunnel when one is
// configured and direct otherwise. A configured VPN never falls back to
// direct: if the tunnel fails, no engine is started.
func newRuntimeFactory(stateDir string, log *zerolog.Logger) engine.Factory {
	return func(ctx context.Context, rs engine.Settings) (*engine.Runtime, error) {
		rt := &engine.Runtime{}
		var n netx.Network = netx.Direct{}
		listenPort := 0
		if rs.VPN != nil {
			conn, err := rs.VPN.Connect(ctx, provider.ConnectOptions{PortForward: true}, log)
			if err != nil {
				return nil, err
			}
			rt.VPN = conn
			n, listenPort = conn.Tunnel, int(conn.ForwardedPort) // 0 = any port
		}
		eng, err := torrent.New(torrent.Config{StateDir: stateDir, Network: n, ListenPort: listenPort, Seed: true}, log)
		if err != nil {
			if rt.VPN != nil {
				rt.VPN.Close()
			}
			return nil, err
		}
		rt.Engine = eng
		rt.Close = func() {
			eng.Close() // stop torrent traffic before its network goes away
			if rt.VPN != nil {
				rt.VPN.Close()
			}
		}
		return rt, nil
	}
}
