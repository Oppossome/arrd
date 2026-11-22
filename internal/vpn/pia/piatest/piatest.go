// Package piatest fakes PIA's sign-in, server list and WireGuard key API
// for tests. Registering a key starts a real in-process WireGuard server
// (see vpntest) for that key, so tunnels provisioned through the fake
// really come up and really go stale when the fake drops its peers.
package piatest

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"generic-arr-project/internal/vpn"
	"generic-arr-project/internal/vpn/pia"
	"generic-arr-project/internal/vpn/vpntest"
)

// Fictional account and regions served by the fake.
const (
	Username     = "p0000001"
	Password     = "fictional-password-1"
	Region       = "zz-fictional" // supports port forwarding
	RegionNoPF   = "yy-fictional" // does not
	ServerName   = "fictional101" // certificate name of the only server
	token        = "fictional-token-abc"
	tunnelDNSStr = "10.99.0.2"
)

// Fake is a running fake PIA.
type Fake struct {
	client *pia.Client

	mu      sync.Mutex
	down    bool
	keys    []vpn.Key
	servers []*vpn.Tunnel
}

// New starts a fake PIA; everything stops when the test ends.
func New(t testing.TB) *Fake {
	t.Helper()
	f := &Fake{}
	pool, cert := newCA(t)

	api := httptest.NewUnstartedServer(http.HandlerFunc(f.addKey))
	api.TLS = &tls.Config{Certificates: []tls.Certificate{cert}}
	api.StartTLS()
	t.Cleanup(api.Close)

	web := httptest.NewServer(http.HandlerFunc(f.web))
	t.Cleanup(web.Close)
	t.Cleanup(f.DropPeers)

	d := &net.Dialer{Timeout: 5 * time.Second}
	f.client = &pia.Client{
		HTTP:          web.Client(),
		Dial:          d.DialContext,
		TokenURL:      web.URL + "/token",
		ServerListURL: web.URL + "/servers",
		RootCAs:       pool,
		APIPort:       api.Listener.Addr().(*net.TCPAddr).Port,
		PFPort:        1, // port forwarding is not faked
	}
	return f
}

// Client returns a PIA client that talks to the fake.
func (f *Fake) Client() *pia.Client { return f.client }

// SetDown makes sign-in fail (as if PIA's API were unreachable).
func (f *Fake) SetDown(down bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.down = down
}

// DropPeers stops every WireGuard server: established tunnels go stale.
func (f *Fake) DropPeers() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.servers {
		s.Close()
	}
	f.servers = nil
}

// Keys returns every public key registered so far.
func (f *Fake) Keys() []vpn.Key {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]vpn.Key(nil), f.keys...)
}

func (f *Fake) isDown() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.down
}

func (f *Fake) web(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/servers":
		region := func(id, name string, pf bool) map[string]any {
			return map[string]any{
				"id": id, "name": name, "country": "ZZ", "port_forward": pf, "offline": false,
				"servers": map[string]any{"wg": []map[string]string{{"ip": "127.0.0.1", "cn": ServerName}}},
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"regions": []any{
			region(Region, "Fictional Bay", true),
			region(RegionNoPF, "Fictional Ridge", false),
		}})
		fmt.Fprintln(w, "ZmFrZSBzaWduYXR1cmU=")
	case "/token":
		if f.isDown() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if err := r.ParseMultipartForm(1 << 16); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.FormValue("username") != Username || r.FormValue("password") != Password {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, `{"token":%q}`, token)
	default:
		http.NotFound(w, r)
	}
}

func (f *Fake) addKey(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if r.URL.Path != "/addKey" || q.Get("pt") != token {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	pub, err := vpn.ParseKey(q.Get("pubkey"))
	if err != nil {
		fmt.Fprint(w, `{"status":"ERROR","message":"bad pubkey"}`)
		return
	}
	server, serverPub, port, err := vpntest.NewServer(r.Context(), pub)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	f.mu.Lock()
	f.keys = append(f.keys, pub)
	f.servers = append(f.servers, server)
	f.mu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": "OK", "server_key": serverPub.String(), "server_port": port,
		"server_ip": "127.0.0.1", "server_vip": "10.99.0.2",
		"peer_ip": vpntest.AddrA.String(), "peer_pubkey": pub.String(),
		"dns_servers": []string{tunnelDNSStr},
	})
}

// newCA makes a throwaway CA and a certificate from it for ServerName.
func newCA(t testing.TB) (*x509.CertPool, tls.Certificate) {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Fictional PIA Test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: ServerName}, DNSNames: []string{ServerName},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return pool, tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}
