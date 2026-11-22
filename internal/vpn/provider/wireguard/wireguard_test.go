package wireguard

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"generic-arr-project/internal/vpn/provider"
	"generic-arr-project/internal/vpn/vpntest"
)

func TestParseAndCheckReportConfigField(t *testing.T) {
	d := Definition()
	for _, cfg := range []string{"", "[Interface]\n", "[Interface]\nPrivateKey = nope*\n"} {
		_, err := d.Parse(provider.Values{"config": cfg})
		var fe *provider.FieldError
		if !errors.As(err, &fe) || fe.Field != "config" {
			t.Errorf("parse %q: err = %v, want field error on config", cfg, err)
		}
		if err := d.Check(context.Background(), provider.Values{"config": cfg}); !errors.As(err, &fe) {
			t.Errorf("check %q: err = %v", cfg, err)
		}
	}
	if f, ok := d.Field("config"); !ok || !f.Secret || !f.Required {
		t.Errorf("config field = %+v: must be a required secret (it holds the private key)", f)
	}
}

func TestConnect(t *testing.T) {
	_, clientConf := vpntest.Server(t)
	p, err := Definition().Parse(provider.Values{"config": clientConf})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := p.Connect(context.Background(), provider.ConnectOptions{PortForward: true}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if conn.Reprovision || conn.Maintain != nil || conn.ForwardedPort != 0 {
		t.Errorf("static connection = %+v: nothing to renew or forward", conn)
	}
	if len(conn.Summary.Addresses) != 1 || conn.Server == "" {
		t.Errorf("summary = %+v, server %q", conn.Summary, conn.Server)
	}
}
