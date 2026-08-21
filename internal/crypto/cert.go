package crypto

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"strings"
	"time"
)

type FileCert struct {
	DNSNames   []string
	Issuer     string
	NotBefore  time.Time
	NotAfter   time.Time
	SelfSigned bool
}

func ReadLeafCert(certPath string) (FileCert, error) {
	raw, err := os.ReadFile(certPath)
	if err != nil {
		return FileCert{}, err
	}
	var leaf *x509.Certificate
	rest := raw
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		leaf = c
		break
	}
	if leaf == nil {
		return FileCert{}, os.ErrNotExist
	}
	iss := strings.TrimSpace(leaf.Issuer.CommonName)
	if iss == "" && len(leaf.Issuer.Organization) > 0 {
		iss = leaf.Issuer.Organization[0]
	}
	return FileCert{
		DNSNames:   append([]string{}, leaf.DNSNames...),
		Issuer:     iss,
		NotBefore:  leaf.NotBefore,
		NotAfter:   leaf.NotAfter,
		SelfSigned: string(leaf.RawIssuer) == string(leaf.RawSubject),
	}, nil
}

// PublicCertTrusts reports whether certPath is a currently valid, non-self-signed
// certificate that covers host (Let's Encrypt or any public CA leaf we stored).
func PublicCertTrusts(certPath, host string) bool {
	c, err := ReadLeafCert(certPath)
	if err != nil || c.SelfSigned {
		return false
	}
	if time.Now().After(c.NotAfter) || time.Now().Before(c.NotBefore.Add(-time.Hour)) {
		return false
	}
	host = strings.ToLower(strings.TrimSpace(host))
	for _, n := range c.DNSNames {
		if strings.EqualFold(n, host) {
			return true
		}
	}
	return false
}
