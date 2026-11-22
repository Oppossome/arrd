package pia_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"generic-arr-project/internal/vpn/pia"
	"generic-arr-project/internal/vpn/pia/piatest"
	"generic-arr-project/internal/vpn/provider"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func goodValues() provider.Values {
	return provider.Values{"username": piatest.Username, "password": piatest.Password, "region": piatest.Region}
}

func TestDefinition(t *testing.T) {
	fake := piatest.New(t)
	d := pia.Definition(fake.Client())
	if _, err := provider.NewRegistry(d); err != nil {
		t.Fatal(err)
	}
	if f, ok := d.Field("password"); !ok || !f.Secret {
		t.Errorf("password field = %+v, must be secret", f)
	}
	if !d.Renews {
		t.Error("PIA renews credentials on every connect")
	}
	opts, err := d.Options(context.Background(), "region")
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range opts {
		pf := slices.Contains(o.Tags, pia.TagPortForward)
		if pf != (o.Value == piatest.Region) || pf != strings.Contains(o.Label, "port forwarding") {
			t.Errorf("option %+v: port forwarding tag/label wrong", o)
		}
	}
	if _, err := d.Options(context.Background(), "username"); err == nil {
		t.Error("options for a text field")
	}
}

func TestCheck(t *testing.T) {
	ctx := context.Background()
	fake := piatest.New(t)
	d := pia.Definition(fake.Client())
	if err := d.Check(ctx, goodValues()); err != nil {
		t.Fatalf("good values: %v", err)
	}
	cases := map[string]struct {
		mutate func(provider.Values)
		field  string
	}{
		"wrong password": {func(v provider.Values) { v["password"] = "nope" }, "password"},
		"unknown region": {func(v provider.Values) { v["region"] = "xx-nowhere" }, "region"},
		"pf unsupported": {func(v provider.Values) { v["region"], v["port_forward"] = piatest.RegionNoPF, "true" }, "port_forward"},
		"missing user":   {func(v provider.Values) { delete(v, "username") }, "username"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			v := goodValues()
			c.mutate(v)
			err := d.Check(ctx, v)
			var fe *provider.FieldError
			if !errors.As(err, &fe) || fe.Field != c.field {
				t.Fatalf("err = %v, want field error on %s", err, c.field)
			}
			if strings.Contains(err.Error(), piatest.Password) {
				t.Errorf("error leaks the password: %v", err)
			}
		})
	}
	fake.SetDown(true)
	var fe *provider.FieldError
	if err := d.Check(ctx, goodValues()); err == nil || errors.As(err, &fe) {
		t.Errorf("PIA down: err = %v, want a non-field error", err)
	}
}

func TestConnect(t *testing.T) {
	ctx := context.Background()
	fake := piatest.New(t)
	d := pia.Definition(fake.Client())

	p, err := d.Parse(goodValues())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := p.Connect(ctx, provider.ConnectOptions{PortForward: true}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if !conn.Reprovision || conn.Server != piatest.ServerName || conn.ForwardedPort != 0 || conn.Maintain != nil {
		t.Errorf("connection = %+v (port forwarding is off in these settings)", conn)
	}
	if len(fake.Keys()) != 1 {
		t.Errorf("registered %d keys", len(fake.Keys()))
	}

	// Port forwarding on, but the server cannot provide it: the tunnel is
	// torn down rather than handed out without the port.
	v := goodValues()
	v["port_forward"] = "true"
	p, err = d.Parse(v)
	if err != nil {
		t.Fatal(err)
	}
	short, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if _, err := p.Connect(short, provider.ConnectOptions{PortForward: true}, quietLog()); err == nil || !strings.Contains(err.Error(), "port forwarding") {
		t.Fatalf("connect with failing port forwarding: err = %v", err)
	}
	// Without the engine on the tunnel no port is requested, so it works.
	conn2, err := p.Connect(ctx, provider.ConnectOptions{}, quietLog())
	if err != nil {
		t.Fatalf("connect without port forwarding: %v", err)
	}
	conn2.Close()

	if _, err := d.Parse(provider.Values{"username": "u", "region": "r"}); err == nil {
		t.Error("parse accepted missing password")
	}
}
