package pia

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Forward is a port PIA forwards to us through the tunnel. PIA drops it
// unless Bind is called at least every 15 minutes; the signature that
// backs it lasts about two months.
type Forward struct {
	Port    uint16
	Expires time.Time

	client    *Client
	dial      DialFunc
	server    Server
	payload   string
	signature string
}

type signatureJSON struct {
	Status    string `json:"status"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

type payloadJSON struct {
	Port      int    `json:"port"`
	ExpiresAt string `json:"expires_at"`
}

// PortForward asks the server for a forwarded port. tunnelDial must route
// through the tunnel to srv: the API only answers on its inside. Call Bind
// before relying on the port.
func (c *Client) PortForward(ctx context.Context, tunnelDial DialFunc, srv Server, token string) (*Forward, error) {
	var sig signatureJSON
	if err := c.pinnedGet(ctx, tunnelDial, srv, c.PFPort, "/getSignature", url.Values{"token": {token}}, &sig); err != nil {
		return nil, err
	}
	if sig.Status != "OK" {
		return nil, fmt.Errorf("pia %s getSignature: status %q", srv.CN, sig.Status)
	}
	if sig.Payload == "" || sig.Signature == "" {
		return nil, fmt.Errorf("pia %s getSignature: missing payload or signature", srv.CN)
	}
	raw, err := base64.StdEncoding.DecodeString(sig.Payload)
	if err != nil {
		return nil, fmt.Errorf("pia %s getSignature: payload: %w", srv.CN, err)
	}
	var p payloadJSON
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("pia %s getSignature: payload: %w", srv.CN, err)
	}
	if p.Port < 1 || p.Port > 65535 {
		return nil, fmt.Errorf("pia %s getSignature: port %d out of range", srv.CN, p.Port)
	}
	expires, err := time.Parse(time.RFC3339Nano, p.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("pia %s getSignature: expires_at: %w", srv.CN, err)
	}
	return &Forward{
		Port: uint16(p.Port), Expires: expires,
		client: c, dial: tunnelDial, server: srv,
		payload: sig.Payload, signature: sig.Signature,
	}, nil
}

// Bind activates or renews the forwarded port.
func (f *Forward) Bind(ctx context.Context) error {
	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	q := url.Values{"payload": {f.payload}, "signature": {f.signature}}
	if err := f.client.pinnedGet(ctx, f.dial, f.server, f.client.PFPort, "/bindPort", q, &out); err != nil {
		return err
	}
	if out.Status != "OK" {
		return fmt.Errorf("pia %s bindPort: status %q: %s", f.server.CN, out.Status, out.Message)
	}
	return nil
}
