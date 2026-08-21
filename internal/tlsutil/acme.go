package tlsutil

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/crypto/acme"
)

func (m *Manager) acmeClient() *acme.Client {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.client != nil {
		return m.client
	}
	m.client = &acme.Client{Key: m.acct, DirectoryURL: acme.LetsEncryptURL}
	return m.client
}

func (m *Manager) issueOne(ctx context.Context, name string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	client := m.acmeClient()
	contact := []string{}
	m.mu.Lock()
	email := m.email
	m.mu.Unlock()
	if email != "" {
		contact = []string{"mailto:" + email}
	}
	acct := &acme.Account{Contact: contact}
	if _, err := client.Register(ctx, acct, acme.AcceptTOS); err != nil && err != acme.ErrAccountAlreadyExists {
		return fmt.Errorf("register: %w", err)
	}
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(name))
	if err != nil {
		return fmt.Errorf("order: %w", err)
	}
	for _, u := range order.AuthzURLs {
		az, err := client.GetAuthorization(ctx, u)
		if err != nil {
			return err
		}
		if az.Status == acme.StatusValid {
			continue
		}
		var chal *acme.Challenge
		for _, c := range az.Challenges {
			if c.Type == "http-01" {
				chal = c
				break
			}
		}
		if chal == nil {
			return fmt.Errorf("no http-01 challenge for %s", name)
		}
		resp, err := client.HTTP01ChallengeResponse(chal.Token)
		if err != nil {
			return err
		}
		m.mu.Lock()
		m.tokens[chal.Token] = resp
		m.mu.Unlock()
		defer func(tok string) {
			m.mu.Lock()
			delete(m.tokens, tok)
			m.mu.Unlock()
		}(chal.Token)
		if _, err := client.Accept(ctx, chal); err != nil {
			return fmt.Errorf("accept challenge: %w", err)
		}
		if _, err := client.WaitAuthorization(ctx, az.URI); err != nil {
			return fmt.Errorf("wait auth %s: %w", name, err)
		}
	}
	if _, err := client.WaitOrder(ctx, order.URI); err != nil {
		return fmt.Errorf("wait order: %w", err)
	}
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:  pkix.Name{CommonName: name},
		DNSNames: []string{name},
	}, certKey)
	if err != nil {
		return err
	}
	der, _, err := client.CreateOrderCert(ctx, order.FinalizeURL, csr, true)
	if err != nil {
		return fmt.Errorf("finalize: %w", err)
	}
	tlsCert := tls.Certificate{Certificate: der, PrivateKey: certKey}
	dir := filepath.Join(m.cacheDir, name)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if err := writeTLSPair(filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem"), &tlsCert); err != nil {
		return err
	}
	hc := wrapCert(name, &tlsCert, "")
	m.mu.Lock()
	m.certs[name] = hc
	m.mu.Unlock()
	return nil
}
