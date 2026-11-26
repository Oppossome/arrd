package app

import (
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"generic-arr-project/internal/testutil"
)

// stubMDNS counts responder starts and stops; it fails to start while err is set.
type stubMDNS struct {
	running int
	err     error
}

func (s *stubMDNS) start() (io.Closer, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.running++
	return closerFunc(func() error { s.running--; return nil }), nil
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

func newTestExposer(t *testing.T, baseAddr string, mdns *stubMDNS) *exposer {
	t.Helper()
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		testutil.Write(t, w, "ok")
	})}
	e := newExposer(srv, baseAddr, mdns.start, new(zerolog.Nop()))
	t.Cleanup(func() {
		_ = srv.Close()
		e.Close()
	})
	return e
}

// boundPort is the port the exposer is listening on.
func boundPort(e *exposer) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return portOf(e.ln.Addr())
}

func boundHost(e *exposer) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	host, _, _ := net.SplitHostPort(e.ln.Addr().String())
	return host
}

func assertServes(t *testing.T, port string) {
	t.Helper()
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer testutil.Close(t, resp.Body)
	if body, _ := io.ReadAll(resp.Body); string(body) != "ok" {
		t.Fatalf("body = %q", body)
	}
}

func TestExposerTogglesLAN(t *testing.T) {
	mdns := &stubMDNS{}
	e := newTestExposer(t, "127.0.0.1:0", mdns)

	if err := e.ApplyLAN(false); err != nil {
		t.Fatal(err)
	}
	if host := boundHost(e); host != "127.0.0.1" {
		t.Errorf("off: bound to %s", host)
	}
	if e.LANURL() != "" || mdns.running != 0 {
		t.Errorf("off: LANURL %q, mDNS running %d", e.LANURL(), mdns.running)
	}
	assertServes(t, boundPort(e))

	if err := e.ApplyLAN(true); err != nil {
		t.Fatal(err)
	}
	if host := boundHost(e); host != "0.0.0.0" && host != "::" {
		t.Errorf("on: bound to %s, want a wildcard address", host)
	}
	if want := "http://arrd.local:" + boundPort(e); e.LANURL() != want || mdns.running != 1 {
		t.Errorf("on: LANURL %q (want %q), mDNS running %d", e.LANURL(), want, mdns.running)
	}
	assertServes(t, boundPort(e))

	if err := e.ApplyLAN(true); err != nil || mdns.running != 1 {
		t.Errorf("re-enable: err %v, mDNS running %d", err, mdns.running)
	}

	if err := e.ApplyLAN(false); err != nil {
		t.Fatal(err)
	}
	if boundHost(e) != "127.0.0.1" || e.LANURL() != "" || mdns.running != 0 {
		t.Errorf("off again: host %s, LANURL %q, mDNS running %d", boundHost(e), e.LANURL(), mdns.running)
	}
	assertServes(t, boundPort(e))

	select {
	case err := <-e.Failed():
		t.Errorf("swapping listeners reported a failure: %v", err)
	default:
	}
}

func TestExposerMDNSFailureKeepsListener(t *testing.T) {
	mdns := &stubMDNS{err: errors.New("no multicast interface")}
	e := newTestExposer(t, "127.0.0.1:0", mdns)
	if err := e.ApplyLAN(false); err != nil {
		t.Fatal(err)
	}

	if err := e.ApplyLAN(true); err == nil {
		t.Fatal("ApplyLAN(true) succeeded without mDNS")
	}
	if boundHost(e) != "127.0.0.1" || e.LANURL() != "" {
		t.Errorf("after failure: host %s, LANURL %q", boundHost(e), e.LANURL())
	}
	assertServes(t, boundPort(e))
}

func TestExposerBindFailureRestoresListener(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := portOf(probe.Addr())
	testutil.Close(t, probe)

	e := newTestExposer(t, net.JoinHostPort("127.0.0.1", port), &stubMDNS{})
	if err := e.ApplyLAN(false); err != nil {
		t.Fatal(err)
	}
	// Another address holding the port blocks the wildcard bind.
	blocker, err := net.Listen("tcp", net.JoinHostPort("127.0.0.2", port))
	if err != nil {
		t.Skipf("127.0.0.2 unavailable: %v", err)
	}
	defer testutil.Close(t, blocker)

	if err := e.ApplyLAN(true); err == nil {
		t.Fatal("ApplyLAN(true) succeeded although the port is taken")
	}
	if boundHost(e) != "127.0.0.1" || boundPort(e) != port || e.LANURL() != "" {
		t.Errorf("after failure: %s:%s, LANURL %q", boundHost(e), boundPort(e), e.LANURL())
	}
	assertServes(t, port)
}

func TestExposerLANAddr(t *testing.T) {
	for base, want := range map[string]string{
		"127.0.0.1:8484": "0.0.0.0:8484",
		"localhost:8484": "0.0.0.0:8484",
		"[::1]:8484":     "[::]:8484",
		":8484":          "0.0.0.0:8484",
	} {
		e := &exposer{baseAddr: base}
		if got := e.addrFor(true); got != want {
			t.Errorf("addrFor(%q) = %q, want %q", base, got, want)
		}
		if got := e.addrFor(false); got != base {
			t.Errorf("addrFor(%q, off) = %q", base, got)
		}
	}
}
