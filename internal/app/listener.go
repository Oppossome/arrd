package app

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	"generic-arr-project/internal/lan"

	"github.com/rs/zerolog"
)

// exposer owns the HTTP server's listener and switches it between the
// configured -listen address and all interfaces ("LAN access"), starting the
// mDNS responder alongside. It implements settings.NetworkApplier.
type exposer struct {
	srv       *http.Server
	baseAddr  string                    // the -listen address
	startMDNS func() (io.Closer, error) // announces lan.Hostname
	log       *zerolog.Logger
	failed    chan error // the server can no longer serve; buffered 1

	mu   sync.Mutex
	ln   net.Listener // nil before the first ApplyLAN or after a failed restore
	lan  bool
	mdns io.Closer
}

func newExposer(srv *http.Server, baseAddr string, startMDNS func() (io.Closer, error), log *zerolog.Logger) *exposer {
	return &exposer{srv: srv, baseAddr: baseAddr, startMDNS: startMDNS, log: log, failed: make(chan error, 1)}
}

// startMDNSResponder is the production startMDNS.
func startMDNSResponder(log *zerolog.Logger) func() (io.Closer, error) {
	return func() (io.Closer, error) {
		return lan.StartResponder(lan.ResponderOptions{Hostname: lan.Hostname, Log: log})
	}
}

// Failed reports a listener that stopped serving for any reason other than
// being swapped out or shut down.
func (e *exposer) Failed() <-chan error { return e.failed }

// Hostname is the mDNS hostname announced while LAN access is on.
func (e *exposer) Hostname() string { return lan.Hostname }

// LANURL is the web UI's address on the local network, or "" while LAN
// access is off.
func (e *exposer) LANURL() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.lan || e.ln == nil {
		return ""
	}
	return "http://" + net.JoinHostPort(lan.Hostname, portOf(e.ln.Addr()))
}

// ApplyLAN serves on all interfaces and announces over mDNS when enabled,
// or on the -listen address otherwise. The old listener is closed before
// the new one is bound (Linux refuses 0.0.0.0:P while 127.0.0.1:P is held);
// open connections are unaffected. On error the previous listener is
// restored.
func (e *exposer) ApplyLAN(enabled bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ln != nil && e.lan == enabled {
		return nil
	}

	prevAddr := ""
	if e.ln != nil {
		prevAddr = e.ln.Addr().String() // the bound port, even if -listen asked for :0
		if err := e.ln.Close(); err != nil {
			return fmt.Errorf("close listener %s: %w", prevAddr, err)
		}
		e.ln = nil
	}

	addr := e.addrFor(enabled)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		err = fmt.Errorf("listen on %s: %w", addr, err)
		return errors.Join(err, e.restore(prevAddr))
	}

	if enabled && e.mdns == nil {
		m, err := e.startMDNS()
		if err != nil {
			_ = ln.Close()
			return errors.Join(err, e.restore(prevAddr))
		}
		e.mdns = m
	}
	if !enabled {
		e.stopMDNS()
	}

	e.ln, e.lan = ln, enabled
	e.log.Info().Str("addr", ln.Addr().String()).Bool("lan", enabled).Msg("http server listening")
	go e.serve(ln)
	return nil
}

// restore rebinds the previous address after a failed switch. If that fails
// too the server is unreachable, which is reported on Failed.
func (e *exposer) restore(prevAddr string) error {
	if prevAddr == "" {
		return nil
	}
	ln, err := net.Listen("tcp", prevAddr)
	if err != nil {
		err = fmt.Errorf("restore listener %s: %w", prevAddr, err)
		e.fail(err)
		return err
	}
	e.ln = ln
	go e.serve(ln)
	return nil
}

func (e *exposer) serve(ln net.Listener) {
	err := e.srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return
	}
	e.mu.Lock()
	swapped := e.ln != ln
	e.mu.Unlock()
	if swapped {
		return // closed by ApplyLAN on purpose
	}
	e.fail(err)
}

func (e *exposer) fail(err error) {
	select {
	case e.failed <- err:
	default: // one failure is enough to stop Run
	}
}

// Close stops the mDNS responder. The listener is closed by the server's
// Shutdown.
func (e *exposer) Close() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopMDNS()
}

func (e *exposer) stopMDNS() {
	if e.mdns == nil {
		return
	}
	if err := e.mdns.Close(); err != nil {
		e.log.Warn().Err(err).Msg("stop mDNS responder")
	}
	e.mdns = nil
}

// addrFor is -listen, or the wildcard address on its port for LAN access
// ("[::]" when -listen is an IPv6 address).
func (e *exposer) addrFor(enabled bool) string {
	if !enabled {
		return e.baseAddr
	}
	host, port, err := net.SplitHostPort(e.baseAddr)
	if err != nil {
		return e.baseAddr // net.Listen reports the malformed address
	}
	if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
		return net.JoinHostPort("::", port)
	}
	return net.JoinHostPort("0.0.0.0", port)
}

func portOf(addr net.Addr) string {
	if tcp, ok := addr.(*net.TCPAddr); ok {
		return fmt.Sprint(tcp.Port)
	}
	_, port, _ := net.SplitHostPort(addr.String())
	return port
}
