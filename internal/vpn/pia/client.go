// Package pia provisions WireGuard tunnels with Private Internet Access.
//
// PIA does not hand out long-lived WireGuard configs. A client signs in for
// a token, registers a freshly generated public key with one VPN server and
// gets back that server's key and an address to use. The registration
// lapses once the tunnel goes unused, so arrd repeats this whenever it
// (re)starts the tunnel. The protocol follows pia-foss/manual-connections.
//
// Trust: sign-in and the server list use public HTTPS. Calls to VPN servers
// dial the IP from the server list directly (no DNS) and accept only a
// certificate issued by PIA's own CA for that server's hostname. A tampered
// server list can therefore at worst point arrd at another genuine PIA
// server. The list's detached signature is not checked: the CA pin is what
// guarantees the peer's identity.
package pia

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"mime/multipart"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"generic-arr-project/internal/closer"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/vpn"
)

// Default endpoints and ports, from pia-foss/manual-connections.
const (
	DefaultTokenURL      = "https://www.privateinternetaccess.com/api/client/v2/token"
	DefaultServerListURL = "https://serverlist.piaservers.net/vpninfo/servers/v6"
	DefaultAPIPort       = 1337  // WireGuard addKey API on each server
	DefaultPFPort        = 19999 // port forwarding API, reached inside the tunnel

	requestTimeout = 15 * time.Second
	maxBody        = 4 << 20 // the server list is ~200 KB
	// keepalive keeps NAT mappings open and makes WireGuard re-handshake
	// at least every two minutes, which the health check relies on.
	keepalive = 25 * time.Second
)

// ErrInvalidCredentials means PIA rejected the username or password.
var ErrInvalidCredentials = errors.New("pia rejected the username or password")

// DialFunc opens a connection; it decides which network carries it.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Client talks to PIA. Fields other than HTTP and Dial may be changed for
// tests; NewClient fills in the real endpoints.
type Client struct {
	// HTTP fetches the token and server list (public web PKI).
	HTTP *http.Client
	// Dial reaches VPN servers' addKey API before the tunnel exists.
	Dial          DialFunc
	TokenURL      string
	ServerListURL string
	RootCAs       *x509.CertPool // trusted for VPN server APIs
	APIPort       int
	PFPort        int
}

// NewClient returns a client whose sign-in, server list and key
// registration use direct (none of them can go through a tunnel that does
// not exist yet; none carry download or indexer traffic).
func NewClient(direct netx.Network) (*Client, error) {
	pool, err := CAPool()
	if err != nil {
		return nil, err
	}
	return &Client{
		HTTP:          netx.HTTPClient(direct, requestTimeout),
		Dial:          direct.DialContext,
		TokenURL:      DefaultTokenURL,
		ServerListURL: DefaultServerListURL,
		RootCAs:       pool,
		APIPort:       DefaultAPIPort,
		PFPort:        DefaultPFPort,
	}, nil
}

// Token signs in and returns a token valid for about a day. Keep it in
// memory only.
func (c *Client) Token(ctx context.Context, username, password string) (_ string, err error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if err := mw.WriteField("username", username); err != nil {
		return "", err
	}
	if err := mw.WriteField("password", password); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, &body)
	if err != nil {
		return "", fmt.Errorf("pia token request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("pia sign-in: %w", err)
	}
	defer closer.Join(&err, resp.Body)
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", ErrInvalidCredentials
	case resp.StatusCode != http.StatusOK:
		return "", fmt.Errorf("pia sign-in: unexpected status %s", resp.Status)
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&out); err != nil {
		return "", fmt.Errorf("pia sign-in: decode response: %w", err)
	}
	if out.Token == "" {
		return "", ErrInvalidCredentials
	}
	return out.Token, nil
}

// Server is one WireGuard server in a region.
type Server struct {
	IP netip.Addr
	CN string // hostname its certificate is issued for
}

// Region is a PIA location with usable WireGuard servers.
type Region struct {
	ID          string
	Name        string
	Country     string
	PortForward bool // servers here can forward a port
	Geo         bool // virtual location (server is elsewhere)
	Servers     []Server
}

type serverListJSON struct {
	Regions []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Country     string `json:"country"`
		PortForward bool   `json:"port_forward"`
		Geo         bool   `json:"geo"`
		Offline     bool   `json:"offline"`
		Servers     struct {
			WG []struct {
				IP string `json:"ip"`
				CN string `json:"cn"`
			} `json:"wg"`
		} `json:"servers"`
	} `json:"regions"`
}

// Regions returns the online regions that have WireGuard servers, sorted
// by name.
func (c *Client) Regions(ctx context.Context) (_ []Region, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.ServerListURL, nil)
	if err != nil {
		return nil, fmt.Errorf("pia server list request: %w", err)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pia server list: %w", err)
	}
	defer closer.Join(&err, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pia server list: unexpected status %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("pia server list: %w", err)
	}
	return parseServerList(raw)
}

// parseServerList reads the JSON document on the first line; the lines
// after it hold a signature.
func parseServerList(raw []byte) ([]Region, error) {
	first, _, _ := bytes.Cut(raw, []byte("\n"))
	var doc serverListJSON
	if err := json.Unmarshal(first, &doc); err != nil {
		return nil, fmt.Errorf("pia server list: decode: %w", err)
	}
	var out []Region
	for _, r := range doc.Regions {
		if r.Offline || r.ID == "" {
			continue
		}
		reg := Region{ID: r.ID, Name: r.Name, Country: r.Country, PortForward: r.PortForward, Geo: r.Geo}
		for _, s := range r.Servers.WG {
			ip, err := netip.ParseAddr(s.IP)
			if err != nil || s.CN == "" {
				continue // unusable entry; others in the region may work
			}
			reg.Servers = append(reg.Servers, Server{IP: ip.Unmap(), CN: s.CN})
		}
		if len(reg.Servers) > 0 {
			out = append(out, reg)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("pia server list: no usable WireGuard regions")
	}
	slices.SortFunc(out, func(a, b Region) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// Peer is what a server returns for a registered key.
type Peer struct {
	ServerKey vpn.Key
	Endpoint  netip.AddrPort // server IP from the list and its WireGuard port
	PeerIP    netip.Addr     // our address inside the tunnel
	DNS       []netip.Addr   // resolvers reachable inside the tunnel
}

type addKeyJSON struct {
	Status     string   `json:"status"`
	ServerKey  string   `json:"server_key"`
	ServerPort int      `json:"server_port"`
	ServerIP   string   `json:"server_ip"`
	PeerIP     string   `json:"peer_ip"`
	PeerPubkey string   `json:"peer_pubkey"`
	DNSServers []string `json:"dns_servers"`
}

// AddKey registers pub with srv and returns the peer details.
func (c *Client) AddKey(ctx context.Context, srv Server, token string, pub vpn.Key) (Peer, error) {
	q := url.Values{"pt": {token}, "pubkey": {pub.String()}}
	var out addKeyJSON
	if err := c.pinnedGet(ctx, c.Dial, srv, c.APIPort, "/addKey", q, &out); err != nil {
		return Peer{}, err
	}
	return out.peer(srv, pub)
}

func (r addKeyJSON) peer(srv Server, pub vpn.Key) (Peer, error) {
	fail := func(format string, args ...any) (Peer, error) {
		return Peer{}, fmt.Errorf("pia %s addKey: "+format, append([]any{srv.CN}, args...)...)
	}
	if r.Status != "OK" {
		return fail("status %q", r.Status)
	}
	key, err := vpn.ParseKey(r.ServerKey)
	if err != nil {
		return fail("server_key: %v", err)
	}
	if r.ServerPort < 1 || r.ServerPort > 65535 {
		return fail("server_port %d out of range", r.ServerPort)
	}
	if r.ServerIP != "" {
		ip, err := netip.ParseAddr(r.ServerIP)
		if err != nil || ip.Unmap() != srv.IP {
			return fail("server_ip %q does not match the server dialed (%s)", r.ServerIP, srv.IP)
		}
	}
	if r.PeerPubkey != "" && r.PeerPubkey != pub.String() {
		return fail("registered a different public key")
	}
	peerIP, err := netip.ParseAddr(r.PeerIP)
	if err != nil {
		return fail("peer_ip: %v", err)
	}
	out := Peer{
		ServerKey: key,
		Endpoint:  netip.AddrPortFrom(srv.IP, uint16(r.ServerPort)),
		PeerIP:    peerIP.Unmap(),
	}
	for _, d := range r.DNSServers {
		a, err := netip.ParseAddr(d)
		if err != nil {
			return fail("dns_servers: %v", err)
		}
		out.DNS = append(out.DNS, a.Unmap())
	}
	return out, nil
}

// pinnedGet calls a VPN server's API at srv.IP:port, trusting only
// c.RootCAs for the name srv.CN, and decodes a JSON reply into out. Errors
// never include the query string: it carries the token.
func (c *Client) pinnedGet(ctx context.Context, dial DialFunc, srv Server, port int, path string, q url.Values, out any) (err error) {
	target := netip.AddrPortFrom(srv.IP, uint16(port)).String()
	hc := &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			Proxy: nil, // a proxy would carry this outside the chosen network
			DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
				return dial(ctx, "tcp", target) // never resolve srv.CN
			},
			TLSClientConfig: &tls.Config{
				RootCAs:    c.RootCAs,
				ServerName: srv.CN,
				MinVersion: tls.VersionTLS12,
			},
			DisableKeepAlives:   true,
			TLSHandshakeTimeout: 10 * time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	u := url.URL{Scheme: "https", Host: net.JoinHostPort(srv.CN, strconv.Itoa(port)), Path: path, RawQuery: q.Encode()}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("pia %s %s: build request", srv.CN, path)
	}
	resp, err := hc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err // drop the URL, which holds the token
		}
		return fmt.Errorf("pia %s (%s) %s: %w", srv.CN, srv.IP, path, err)
	}
	defer closer.Join(&err, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pia %s %s: unexpected status %s", srv.CN, path, resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(out); err != nil {
		return fmt.Errorf("pia %s %s: decode response: %w", srv.CN, path, err)
	}
	return nil
}

// Session is a provisioned tunnel config and what port forwarding needs.
type Session struct {
	Config *vpn.Config
	Server Server
	Token  string // keep in memory only
}

// Provision signs in, registers a new key with a server in region (trying
// the region's servers in random order) and returns a ready tunnel config.
// The private key is generated here and exists only in the returned config.
func (c *Client) Provision(ctx context.Context, username, password string, region Region) (*Session, error) {
	token, err := c.Token(ctx, username, password)
	if err != nil {
		return nil, err
	}
	servers := slices.Clone(region.Servers)
	rand.Shuffle(len(servers), func(i, j int) { servers[i], servers[j] = servers[j], servers[i] })
	var errs []error
	for _, srv := range servers {
		priv, pub, err := vpn.GenerateKeyPair()
		if err != nil {
			return nil, err
		}
		peer, err := c.AddKey(ctx, srv, token, pub)
		if err != nil {
			errs = append(errs, err)
			if ctx.Err() != nil {
				break
			}
			continue
		}
		return &Session{Config: tunnelConfig(priv, peer), Server: srv, Token: token}, nil
	}
	return nil, fmt.Errorf("pia region %s: no server accepted a key: %w", region.ID, errors.Join(errs...))
}

func tunnelConfig(priv vpn.Key, p Peer) *vpn.Config {
	return &vpn.Config{
		PrivateKey: priv,
		Addresses:  []netip.Prefix{netip.PrefixFrom(p.PeerIP, p.PeerIP.BitLen())},
		DNS:        p.DNS,
		MTU:        vpn.DefaultMTU,
		Peers: []vpn.PeerConfig{{
			PublicKey:           p.ServerKey,
			Endpoint:            p.Endpoint.String(),
			AllowedIPs:          []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")},
			PersistentKeepalive: keepalive,
		}},
	}
}

// FindRegion returns the region with id.
func FindRegion(regions []Region, id string) (Region, bool) {
	for _, r := range regions {
		if r.ID == id {
			return r, true
		}
	}
	return Region{}, false
}
