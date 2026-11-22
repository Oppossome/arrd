package pia

import (
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"fmt"
)

// caPEM is PIA's root certificate for its VPN servers' APIs, copied from
// github.com/pia-foss/manual-connections (ca.rsa.4096.crt). Its fingerprint
// is checked by a test so an accidental edit cannot go unnoticed.
//
//go:embed ca.rsa.4096.crt
var caPEM []byte

// CAPool returns a pool holding only PIA's CA. Requests to VPN servers
// trust nothing else, so a server can only answer if PIA issued its
// certificate for the exact hostname we asked for.
func CAPool() (*x509.CertPool, error) {
	cert, err := caCert()
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return pool, nil
}

func caCert() (*x509.Certificate, error) {
	block, _ := pem.Decode(caPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("pia: embedded CA is not a PEM certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pia: parse embedded CA: %w", err)
	}
	return cert, nil
}
