// Package vpn runs a userspace WireGuard tunnel (wireguard-go + gVisor
// netstack) inside the process. Sockets created through a Tunnel only ever
// send traffic through WireGuard; nothing touches the host's routing table.
package vpn

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"
)

// DefaultMTU matches wg-quick's default for most links.
const DefaultMTU = 1420

// Config is a parsed wg-quick style configuration.
type Config struct {
	PrivateKey Key
	Addresses  []netip.Prefix
	DNS        []netip.Addr
	MTU        int
	ListenPort uint16 // local UDP port for WireGuard; 0 picks a random port
	Peers      []PeerConfig
	// IgnoredKeys lists wg-quick keys that have no meaning for an in-process
	// tunnel (PostUp, Table, ...). Callers should surface them to the user.
	IgnoredKeys []string
}

// PeerConfig is one [Peer] section.
type PeerConfig struct {
	PublicKey           Key
	PresharedKey        *Key
	Endpoint            string // host:port; host may be a name
	AllowedIPs          []netip.Prefix
	PersistentKeepalive time.Duration
}

// Key is a 32-byte WireGuard key.
type Key [32]byte

// ParseKey decodes a base64 WireGuard key.
func ParseKey(s string) (Key, error) {
	var k Key
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return k, fmt.Errorf("decode key: %w", err)
	}
	if len(raw) != len(k) {
		return k, fmt.Errorf("key is %d bytes, want %d", len(raw), len(k))
	}
	copy(k[:], raw)
	return k, nil
}

// Hex returns the key in the hex form used by the UAPI protocol.
func (k Key) Hex() string { return hex.EncodeToString(k[:]) }

// String returns the key in the base64 form used by wg-quick and providers.
func (k Key) String() string { return base64.StdEncoding.EncodeToString(k[:]) }

// GenerateKeyPair returns a new random Curve25519 private key and its
// public key.
func GenerateKeyPair() (priv, pub Key, err error) {
	if _, err := rand.Read(priv[:]); err != nil {
		return Key{}, Key{}, fmt.Errorf("generate wireguard key: %w", err)
	}
	// Clamp as required for Curve25519 private keys.
	priv[0] &= 248
	priv[31] = (priv[31] & 127) | 64
	p, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return Key{}, Key{}, fmt.Errorf("derive wireguard public key: %w", err)
	}
	copy(pub[:], p)
	return priv, pub, nil
}

// keys that wg-quick understands but which do not apply to a netstack tunnel.
var ignorableInterfaceKeys = map[string]bool{
	"table": true, "fwmark": true, "saveconfig": true,
	"preup": true, "postup": true, "predown": true, "postdown": true,
}

// ParseConfig parses a wg-quick configuration.
func ParseConfig(r io.Reader) (*Config, error) {
	cfg := &Config{MTU: DefaultMTU}
	var section string
	var peer *PeerConfig
	sawPrivateKey := false

	sc := bufio.NewScanner(r)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			switch section {
			case "interface":
			case "peer":
				cfg.Peers = append(cfg.Peers, PeerConfig{})
				peer = &cfg.Peers[len(cfg.Peers)-1]
			default:
				return nil, fmt.Errorf("line %d: unknown section [%s]", lineNo, section)
			}
			continue
		}
		rawKey, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = value", lineNo)
		}
		key := strings.ToLower(strings.TrimSpace(rawKey))
		value = strings.TrimSpace(value)

		var err error
		switch section {
		case "interface":
			if key == "privatekey" {
				sawPrivateKey = true
			}
			err = cfg.setInterfaceKey(key, strings.TrimSpace(rawKey), value)
		case "peer":
			err = peer.setKey(key, value)
		default:
			err = errors.New("key outside of a section")
		}
		if err != nil {
			return nil, fmt.Errorf("line %d (%s): %w", lineNo, strings.TrimSpace(rawKey), err)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	if !sawPrivateKey {
		return nil, errors.New("[Interface] PrivateKey is required")
	}
	if len(cfg.Addresses) == 0 {
		return nil, errors.New("[Interface] Address is required")
	}
	if len(cfg.Peers) == 0 {
		return nil, errors.New("at least one [Peer] is required")
	}
	for i, p := range cfg.Peers {
		if p.PublicKey == (Key{}) {
			return nil, fmt.Errorf("peer %d: PublicKey is required", i)
		}
		if p.Endpoint == "" {
			return nil, fmt.Errorf("peer %d: Endpoint is required", i)
		}
		if len(p.AllowedIPs) == 0 {
			return nil, fmt.Errorf("peer %d: AllowedIPs is required", i)
		}
	}
	return cfg, nil
}

func (c *Config) setInterfaceKey(key, original, value string) error {
	switch key {
	case "privatekey":
		k, err := ParseKey(value)
		c.PrivateKey = k
		return err
	case "address":
		for _, s := range splitList(value) {
			p, err := parsePrefixOrAddr(s)
			if err != nil {
				return err
			}
			c.Addresses = append(c.Addresses, p)
		}
	case "dns":
		for _, s := range splitList(value) {
			a, err := netip.ParseAddr(s)
			if err != nil {
				// wg-quick allows search domains here; they need a host
				// resolver, which the tunnel does not have.
				c.IgnoredKeys = append(c.IgnoredKeys, "DNS search domain "+s)
				continue
			}
			c.DNS = append(c.DNS, a)
		}
	case "listenport":
		port, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("invalid ListenPort %q", value)
		}
		c.ListenPort = uint16(port)
	case "mtu":
		mtu, err := strconv.Atoi(value)
		if err != nil || mtu < 576 || mtu > 65535 {
			return fmt.Errorf("invalid MTU %q", value)
		}
		c.MTU = mtu
	default:
		if !ignorableInterfaceKeys[key] {
			return fmt.Errorf("unknown [Interface] key")
		}
		c.IgnoredKeys = append(c.IgnoredKeys, original)
	}
	return nil
}

func (p *PeerConfig) setKey(key, value string) error {
	switch key {
	case "publickey":
		k, err := ParseKey(value)
		p.PublicKey = k
		return err
	case "presharedkey":
		k, err := ParseKey(value)
		p.PresharedKey = &k
		return err
	case "endpoint":
		if _, _, err := splitHostPort(value); err != nil {
			return err
		}
		p.Endpoint = value
	case "allowedips":
		for _, s := range splitList(value) {
			pfx, err := netip.ParsePrefix(s)
			if err != nil {
				return err
			}
			p.AllowedIPs = append(p.AllowedIPs, pfx)
		}
	case "persistentkeepalive":
		if value == "off" {
			return nil
		}
		secs, err := strconv.Atoi(value)
		if err != nil || secs < 0 || secs > 65535 {
			return fmt.Errorf("invalid PersistentKeepalive %q", value)
		}
		p.PersistentKeepalive = time.Duration(secs) * time.Second
	default:
		return errors.New("unknown [Peer] key")
	}
	return nil
}

// HasIPv4 reports whether the interface has an IPv4 address.
func (c *Config) HasIPv4() bool { return c.hasFamily(netip.Addr.Is4) }

// HasIPv6 reports whether the interface has an IPv6 address.
func (c *Config) HasIPv6() bool { return c.hasFamily(netip.Addr.Is6) }

func (c *Config) hasFamily(is func(netip.Addr) bool) bool {
	for _, p := range c.Addresses {
		if is(p.Addr()) {
			return true
		}
	}
	return false
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// parsePrefixOrAddr accepts "10.0.0.2/32" or a bare "10.0.0.2".
func parsePrefixOrAddr(s string) (netip.Prefix, error) {
	if strings.Contains(s, "/") {
		return netip.ParsePrefix(s)
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(a, a.BitLen()), nil
}

func splitHostPort(hostport string) (string, uint16, error) {
	i := strings.LastIndexByte(hostport, ':')
	if i <= 0 {
		return "", 0, fmt.Errorf("endpoint %q: missing port", hostport)
	}
	host := strings.TrimSuffix(strings.TrimPrefix(hostport[:i], "["), "]")
	port, err := strconv.ParseUint(hostport[i+1:], 10, 16)
	if err != nil || port == 0 {
		return "", 0, fmt.Errorf("endpoint %q: invalid port", hostport)
	}
	return host, uint16(port), nil
}
