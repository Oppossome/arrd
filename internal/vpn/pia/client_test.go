package pia

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"generic-arr-project/internal/testutil"
	"generic-arr-project/internal/vpn"
)

// pinnedCAFingerprint is the SHA-256 of the embedded CA's DER bytes, as
// published in pia-foss/manual-connections.
const pinnedCAFingerprint = "1fd25658456eab3041fba77ccd398ab8124edcc1b8b2fc1d55fdf6b1bbfc9d70"

func TestEmbeddedCAFingerprint(t *testing.T) {
	cert, err := caCert()
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(cert.Raw)
	if got := hex.EncodeToString(sum[:]); got != pinnedCAFingerprint {
		t.Fatalf("embedded CA fingerprint = %s, want %s", got, pinnedCAFingerprint)
	}
	if !cert.IsCA {
		t.Error("embedded certificate is not a CA")
	}
}

// testCA is a throwaway CA that issues a server certificate for one name.
type testCA struct {
	pool *x509.CertPool
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Fictional Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &testCA{pool: pool, cert: cert, key: key}
}

func (ca *testCA) serverCert(t *testing.T, name string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: name},
		DNSNames:     []string{name},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

const (
	testCN    = "fictionalserver7"
	testToken = "tok-secret-1234"
)

// fakeVPNServer is a TLS server standing in for a PIA server's APIs. It
// listens on 127.0.0.1 with a certificate for testCN from ca.
func fakeVPNServer(t *testing.T, ca *testCA, h http.Handler) (Server, int) {
	t.Helper()
	srv := httptest.NewUnstartedServer(h)
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{ca.serverCert(t, testCN)}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	return Server{IP: netip.MustParseAddr("127.0.0.1"), CN: testCN}, port
}

func testClient(ca *testCA, apiPort int) *Client {
	d := &net.Dialer{Timeout: 5 * time.Second}
	return &Client{
		HTTP:    &http.Client{Timeout: 5 * time.Second},
		Dial:    d.DialContext,
		RootCAs: ca.pool,
		APIPort: apiPort,
		PFPort:  apiPort,
	}
}

func mustKey(t *testing.T) vpn.Key {
	t.Helper()
	_, pub, err := vpn.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	return pub
}

func TestToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if err := r.ParseMultipartForm(1 << 16); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.FormValue("username") == "p0000001" && r.FormValue("password") == "hunter2-fictional" {
			testutil.Write(t, w, `{"token":"`+testToken+`"}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer ts.Close()
	c := &Client{HTTP: ts.Client(), TokenURL: ts.URL}
	ctx := context.Background()

	tok, err := c.Token(ctx, "p0000001", "hunter2-fictional")
	if err != nil || tok != testToken {
		t.Fatalf("Token = %q, %v", tok, err)
	}
	if _, err := c.Token(ctx, "p0000001", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("bad password: err = %v, want ErrInvalidCredentials", err)
	}
}

const serverListFixture = `{"groups":{},"regions":[` +
	`{"id":"zz-lake","name":"Zed Lake","country":"ZZ","port_forward":true,"geo":false,"offline":false,"servers":{"wg":[{"ip":"192.0.2.10","cn":"lake101"},{"ip":"not-an-ip","cn":"lake102"}]}},` +
	`{"id":"aa-hill","name":"Ay Hill","country":"AA","port_forward":false,"geo":true,"offline":false,"servers":{"wg":[{"ip":"192.0.2.20","cn":"hill201"}]}},` +
	`{"id":"bb-down","name":"Bee Down","country":"BB","port_forward":true,"offline":true,"servers":{"wg":[{"ip":"192.0.2.30","cn":"down301"}]}},` +
	`{"id":"cc-nowg","name":"Cee None","country":"CC","port_forward":true,"offline":false,"servers":{"ovpnudp":[{"ip":"192.0.2.40","cn":"none401"}]}}` +
	`]}
c2lnbmF0dXJlIGxpbmUgb25l
c2lnbmF0dXJlIGxpbmUgdHdv
`

func TestRegions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testutil.Write(t, w, serverListFixture)
	}))
	defer ts.Close()
	c := &Client{HTTP: ts.Client(), ServerListURL: ts.URL}
	regions, err := c.Regions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Offline regions and regions without WireGuard servers are dropped;
	// the rest are sorted by name.
	if len(regions) != 2 || regions[0].ID != "aa-hill" || regions[1].ID != "zz-lake" {
		t.Fatalf("regions = %+v", regions)
	}
	lake := regions[1]
	if !lake.PortForward || len(lake.Servers) != 1 || lake.Servers[0] != (Server{IP: netip.MustParseAddr("192.0.2.10"), CN: "lake101"}) {
		t.Errorf("lake = %+v", lake)
	}
	if !regions[0].Geo || regions[0].PortForward {
		t.Errorf("hill = %+v", regions[0])
	}
	if _, ok := FindRegion(regions, "bb-down"); ok {
		t.Error("offline region found")
	}
}

func addKeyHandler(t *testing.T, resp func(pub string) map[string]any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/addKey" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("pt") != testToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if err := json.NewEncoder(w).Encode(resp(r.URL.Query().Get("pubkey"))); err != nil {
			t.Error(err)
		}
	})
}

func okAddKey(serverKey vpn.Key) func(string) map[string]any {
	return func(pub string) map[string]any {
		return map[string]any{
			"status": "OK", "server_key": serverKey.String(), "server_port": 1337,
			"server_ip": "127.0.0.1", "server_vip": "10.6.128.1", "peer_ip": "10.6.140.7",
			"peer_pubkey": pub, "dns_servers": []string{"10.0.0.243", "10.0.0.242"},
		}
	}
}

func TestAddKey(t *testing.T) {
	ctx := context.Background()
	ca := newTestCA(t)
	serverKey := mustKey(t)
	srv, port := fakeVPNServer(t, ca, addKeyHandler(t, okAddKey(serverKey)))
	c := testClient(ca, port)
	pub := mustKey(t)

	peer, err := c.AddKey(ctx, srv, testToken, pub)
	if err != nil {
		t.Fatal(err)
	}
	want := Peer{
		ServerKey: serverKey,
		Endpoint:  netip.MustParseAddrPort("127.0.0.1:1337"),
		PeerIP:    netip.MustParseAddr("10.6.140.7"),
		DNS:       []netip.Addr{netip.MustParseAddr("10.0.0.243"), netip.MustParseAddr("10.0.0.242")},
	}
	if fmt.Sprint(peer) != fmt.Sprint(want) {
		t.Errorf("peer = %+v, want %+v", peer, want)
	}

	t.Run("wrong hostname", func(t *testing.T) {
		other := srv
		other.CN = "fictionalserver8"
		_, err := c.AddKey(ctx, other, testToken, pub)
		if err == nil || !strings.Contains(err.Error(), "certificate") {
			t.Fatalf("err = %v, want certificate error", err)
		}
		if strings.Contains(err.Error(), testToken) {
			t.Errorf("error leaks the token: %v", err)
		}
	})
	t.Run("untrusted CA", func(t *testing.T) {
		c2 := testClient(newTestCA(t), port)
		if _, err := c2.AddKey(ctx, srv, testToken, pub); err == nil || !strings.Contains(err.Error(), "certificate") {
			t.Fatalf("err = %v, want certificate error", err)
		}
	})
	t.Run("real PIA CA does not trust the fake", func(t *testing.T) {
		pool, err := CAPool()
		if err != nil {
			t.Fatal(err)
		}
		c2 := testClient(ca, port)
		c2.RootCAs = pool
		if _, err := c2.AddKey(ctx, srv, testToken, pub); err == nil {
			t.Fatal("fake server accepted with the real PIA CA")
		}
	})
}

func TestAddKeyRejectsBadResponses(t *testing.T) {
	ctx := context.Background()
	ca := newTestCA(t)
	serverKey := mustKey(t)
	cases := map[string]func(m map[string]any){
		"status":         func(m map[string]any) { m["status"] = "ERROR" },
		"bad server key": func(m map[string]any) { m["server_key"] = "c2hvcnQ=" },
		"bad port":       func(m map[string]any) { m["server_port"] = 0 },
		"other ip":       func(m map[string]any) { m["server_ip"] = "192.0.2.99" },
		"other pubkey":   func(m map[string]any) { m["peer_pubkey"] = serverKey.String() },
		"bad peer ip":    func(m map[string]any) { m["peer_ip"] = "nope" },
		"bad dns":        func(m map[string]any) { m["dns_servers"] = []string{"nope"} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			srv, port := fakeVPNServer(t, ca, addKeyHandler(t, func(pub string) map[string]any {
				m := okAddKey(serverKey)(pub)
				mutate(m)
				return m
			}))
			if _, err := testClient(ca, port).AddKey(ctx, srv, testToken, mustKey(t)); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestProvisionFailsOverAndBuildsConfig(t *testing.T) {
	ctx := context.Background()
	ca := newTestCA(t)
	serverKey := mustKey(t)
	var registered []string
	srv, port := fakeVPNServer(t, ca, addKeyHandler(t, func(pub string) map[string]any {
		registered = append(registered, pub)
		return okAddKey(serverKey)(pub)
	}))
	tokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		testutil.Write(t, w, `{"token":"`+testToken+`"}`)
	}))
	defer tokens.Close()
	c := testClient(ca, port)
	c.HTTP, c.TokenURL = tokens.Client(), tokens.URL

	// Nothing listens on 127.0.0.2, so whichever order the servers are
	// tried in, provisioning ends on the working one.
	dead := Server{IP: netip.MustParseAddr("127.0.0.2"), CN: testCN}
	region := Region{ID: "zz-lake", Servers: []Server{dead, srv}}
	sess, err := c.Provision(ctx, "p0000001", "pw", region)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Server != srv || sess.Token != testToken {
		t.Errorf("session server/token = %+v / %q", sess.Server, sess.Token)
	}
	cfg := sess.Config
	if len(registered) != 1 || cfg.PrivateKey == (vpn.Key{}) {
		t.Fatalf("registered %v", registered)
	}
	if len(cfg.Peers) != 1 || cfg.Peers[0].PublicKey != serverKey || cfg.Peers[0].Endpoint != "127.0.0.1:1337" ||
		cfg.Peers[0].PersistentKeepalive != keepalive || cfg.Peers[0].AllowedIPs[0].String() != "0.0.0.0/0" {
		t.Errorf("peer = %+v", cfg.Peers)
	}
	if cfg.Addresses[0].String() != "10.6.140.7/32" || len(cfg.DNS) != 2 {
		t.Errorf("interface = %v dns %v", cfg.Addresses, cfg.DNS)
	}

	// A second provisioning uses a brand-new key.
	sess2, err := c.Provision(ctx, "p0000001", "pw", Region{ID: "zz-lake", Servers: []Server{srv}})
	if err != nil {
		t.Fatal(err)
	}
	if sess2.Config.PrivateKey == cfg.PrivateKey || registered[0] == registered[1] {
		t.Error("key reused across provisioning")
	}

	if _, err := c.Provision(ctx, "p0000001", "pw", Region{ID: "zz-gone", Servers: []Server{dead}}); err == nil {
		t.Error("provisioned with no reachable server")
	}
}

func TestPortForward(t *testing.T) {
	ctx := context.Background()
	ca := newTestCA(t)
	expires := time.Date(2027, 1, 2, 3, 4, 5, 600, time.UTC)
	payload := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(
		`{"token":"x","port":47123,"expires_at":%q}`, expires.Format(time.RFC3339Nano))))
	binds := 0
	srv, port := fakeVPNServer(t, ca, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/getSignature":
			if q.Get("token") != testToken {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			testutil.Write(t, w, fmt.Sprintf(`{"status":"OK","payload":%q,"signature":"sig"}`, payload))
		case "/bindPort":
			if q.Get("payload") != payload || q.Get("signature") != "sig" {
				testutil.Write(t, w, `{"status":"ERROR","message":"bad signature"}`)
				return
			}
			binds++
			testutil.Write(t, w, `{"status":"OK","message":"port scheduled for add"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	c := testClient(ca, port)
	tunnelDial := (&net.Dialer{}).DialContext // stands in for the tunnel

	fw, err := c.PortForward(ctx, tunnelDial, srv, testToken)
	if err != nil {
		t.Fatal(err)
	}
	if fw.Port != 47123 || !fw.Expires.Equal(expires) {
		t.Errorf("forward = %d until %v", fw.Port, fw.Expires)
	}
	for range 2 {
		if err := fw.Bind(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if binds != 2 {
		t.Errorf("binds = %d", binds)
	}
	fw.signature = "forged"
	if err := fw.Bind(ctx); err == nil {
		t.Error("bind with a bad signature succeeded")
	}
	if _, err := c.PortForward(ctx, tunnelDial, srv, "wrong-token"); err == nil {
		t.Error("getSignature with a bad token succeeded")
	}
}

func TestPortKeeper(t *testing.T) {
	ctx := context.Background()
	ca := newTestCA(t)
	now := time.Now()
	expires := now.Add(60 * 24 * time.Hour)
	payload := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`{"port":51413,"expires_at":%q}`, expires.Format(time.RFC3339Nano))))
	binds, failBind := 0, false
	srv, port := fakeVPNServer(t, ca, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/getSignature":
			testutil.Write(t, w, fmt.Sprintf(`{"status":"OK","payload":%q,"signature":"sig"}`, payload))
		case "/bindPort":
			if failBind {
				testutil.Write(t, w, `{"status":"ERROR","message":"port expired"}`)
				return
			}
			binds++
			testutil.Write(t, w, `{"status":"OK"}`)
		}
	}))
	c := testClient(ca, port)
	fw, err := c.PortForward(ctx, (&net.Dialer{}).DialContext, srv, testToken)
	if err != nil {
		t.Fatal(err)
	}
	k := &portKeeper{fw: fw, lastBind: now}

	if err := k.maintain(ctx, now.Add(time.Minute)); err != nil || binds != 0 {
		t.Fatalf("early maintain: err %v, binds %d", err, binds)
	}
	if err := k.maintain(ctx, now.Add(bindEvery)); err != nil || binds != 1 {
		t.Fatalf("due maintain: err %v, binds %d", err, binds)
	}
	if err := k.maintain(ctx, now.Add(bindEvery+time.Minute)); err != nil || binds != 1 {
		t.Fatalf("maintain right after a bind: err %v, binds %d", err, binds)
	}
	failBind = true
	if err := k.maintain(ctx, now.Add(3*bindEvery)); err == nil {
		t.Fatal("failed bind not reported")
	}
	if err := k.maintain(ctx, expires.Add(-renewBefore/2)); err == nil || !strings.Contains(err.Error(), "expires") {
		t.Fatalf("near expiry: err = %v", err)
	}
}
