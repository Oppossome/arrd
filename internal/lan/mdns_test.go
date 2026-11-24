package lan

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/pion/mdns/v2"
	"golang.org/x/net/ipv4"
)

func TestResponderAnswersQueries(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	const name = "arrd-test.local" // not Hostname, so a running arrd cannot answer instead
	r, err := StartResponder(ResponderOptions{Hostname: name, Log: log, IncludeLoopback: true})
	if err != nil {
		t.Skipf("mDNS unavailable here: %v", err)
	}
	defer r.Close()

	lc := net.ListenConfig{Control: reuseAddr}
	l, err := lc.ListenPacket(context.Background(), "udp4", net.JoinHostPort("0.0.0.0", mdnsPort))
	if err != nil {
		t.Skipf("mDNS client socket unavailable: %v", err)
	}
	client, err := mdns.Server(ipv4.NewPacketConn(l), nil, &mdns.Config{IncludeLoopback: true, LoggerFactory: slogFactory{log}})
	if err != nil {
		t.Skipf("mDNS client unavailable: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, addr, err := client.QueryAddr(ctx, name)
	if err != nil {
		t.Fatalf("query %s: %v", name, err)
	}
	if !addr.IsValid() {
		t.Errorf("query %s answered with no address", name)
	}

	if _, err := StartResponder(ResponderOptions{Log: log}); err == nil {
		t.Error("StartResponder without a hostname succeeded")
	}
}
