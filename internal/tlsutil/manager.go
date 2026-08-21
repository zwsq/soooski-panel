package tlsutil

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/zwsq/soooski-panel/internal/crypto"
	"github.com/zwsq/soooski-panel/internal/models"
	"golang.org/x/crypto/acme"
)

const renewWindow = 30 * 24 * time.Hour

type hostCert struct {
	tls       *tls.Certificate
	issuer    string
	notBefore time.Time
	notAfter  time.Time
	err       string
}

// Manager issues Let's Encrypt certificates per configured hostname (HTTP-01),
// serves them on the panel TLS stack, copies the primary one to the sing-box
// cert paths, and renews about 30 days before expiry.
type Manager struct {
	email    string
	cacheDir string
	certPath string
	keyPath  string
	hosts    func() []string
	onChange func()

	mu      sync.Mutex
	tokens  map[string]string
	certs   map[string]*hostCert
	client  *acme.Client
	acct    *ecdsa.PrivateKey
	issuing bool
}

func New(email, cacheDir, certFile, keyFile string, hosts func() []string) (*Manager, error) {
	m := &Manager{
		email:    strings.TrimSpace(email),
		cacheDir: cacheDir,
		certPath: certFile,
		keyPath:  keyFile,
		hosts:    hosts,
		tokens:   map[string]string{},
		certs:    map[string]*hostCert{},
	}
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return nil, err
	}
	if err := m.loadAccount(); err != nil {
		return nil, err
	}
	m.loadCached()
	return m, nil
}

func (m *Manager) SetEmail(email string) {
	m.mu.Lock()
	m.email = strings.TrimSpace(email)
	m.mu.Unlock()
}

func (m *Manager) SetOnChange(fn func()) { m.onChange = fn }

func (m *Manager) loadAccount() error {
	p := filepath.Join(m.cacheDir, "account.key")
	raw, err := os.ReadFile(p)
	if err == nil {
		block, _ := pem.Decode(raw)
		if block != nil {
			k, err := x509.ParseECPrivateKey(block.Bytes)
			if err == nil {
				m.acct = k
				return nil
			}
		}
	}
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	der, err := x509.MarshalECPrivateKey(k)
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), 0600); err != nil {
		return err
	}
	m.acct = k
	return nil
}

func (m *Manager) loadCached() {
	entries, err := os.ReadDir(m.cacheDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		host := e.Name()
		certFile := filepath.Join(m.cacheDir, host, "cert.pem")
		keyFile := filepath.Join(m.cacheDir, host, "key.pem")
		tlsCert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			continue
		}
		hc := wrapCert(host, &tlsCert, "")
		if hc == nil {
			continue
		}
		m.certs[normalizeHost(host)] = hc
	}
	m.publishPrimary()
}

func wrapCert(host string, tlsCert *tls.Certificate, issueErr string) *hostCert {
	hc := &hostCert{tls: tlsCert, err: issueErr}
	if tlsCert != nil && len(tlsCert.Certificate) > 0 {
		if leaf, err := x509.ParseCertificate(tlsCert.Certificate[0]); err == nil {
			tlsCert.Leaf = leaf
			iss := leaf.Issuer.CommonName
			if iss == "" && len(leaf.Issuer.Organization) > 0 {
				iss = leaf.Issuer.Organization[0]
			}
			hc.issuer = iss
			hc.notBefore = leaf.NotBefore
			hc.notAfter = leaf.NotAfter
		}
	}
	return hc
}

func (m *Manager) names() []string {
	if m.hosts == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, h := range m.hosts() {
		h = normalizeHost(h)
		if !Eligible(h) || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

func (m *Manager) Status() []models.CertStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	raw := []string{}
	if m.hosts != nil {
		raw = m.hosts()
	}
	seen := map[string]bool{}
	var hosts []string
	for _, h := range raw {
		h = normalizeHost(h)
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		hosts = append(hosts, h)
	}
	if len(hosts) == 0 {
		fb, err := crypto.ReadLeafCert(m.certPath)
		st := models.CertStatus{Domain: "(none configured)", State: "self_signed", AutoRenew: false}
		if err == nil {
			st.Issuer = fb.Issuer
			st.NotBefore = fb.NotBefore.UTC().Format(time.RFC3339)
			st.NotAfter = fb.NotAfter.UTC().Format(time.RFC3339)
			st.DaysLeft = int(time.Until(fb.NotAfter).Hours() / 24)
		}
		return []models.CertStatus{st}
	}
	var out []models.CertStatus
	for _, h := range hosts {
		cs := models.CertStatus{Domain: h, AutoRenew: Eligible(h)}
		if !Eligible(h) {
			cs.State = "ineligible"
			cs.Error = "IP, localhost, or placeholder — set a real hostname"
			out = append(out, cs)
			continue
		}
		hc := m.certs[h]
		if hc == nil || hc.tls == nil {
			cs.State = "missing"
			if hc != nil {
				cs.Error = hc.err
				if hc.err != "" {
					cs.State = "failed"
				}
			}
			if m.issuing {
				cs.State = "pending"
			}
			out = append(out, cs)
			continue
		}
		cs.Issuer = hc.issuer
		if !hc.notBefore.IsZero() {
			cs.NotBefore = hc.notBefore.UTC().Format(time.RFC3339)
		}
		if !hc.notAfter.IsZero() {
			cs.NotAfter = hc.notAfter.UTC().Format(time.RFC3339)
			cs.DaysLeft = int(time.Until(hc.notAfter).Hours() / 24)
		}
		switch {
		case hc.err != "" && (hc.notAfter.IsZero() || time.Now().After(hc.notAfter)):
			cs.State = "failed"
			cs.Error = hc.err
		case time.Now().After(hc.notAfter):
			cs.State = "failed"
			cs.Error = "certificate expired"
		case time.Until(hc.notAfter) < renewWindow:
			cs.State = "renewing"
		default:
			cs.State = "issued"
		}
		out = append(out, cs)
	}
	return out
}

func (m *Manager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := normalizeHost(hello.ServerName)
	m.mu.Lock()
	hc := m.certs[host]
	m.mu.Unlock()
	if hc != nil && hc.tls != nil && (hc.notAfter.IsZero() || time.Now().Before(hc.notAfter.Add(24*time.Hour))) {
		return hc.tls, nil
	}
	if cert, err := tls.LoadX509KeyPair(m.certPath, m.keyPath); err == nil {
		return &cert, nil
	}
	return nil, fmt.Errorf("no certificate for %s", hello.ServerName)
}

func (m *Manager) TLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: m.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1", acme.ALPNProto},
	}
}

func (m *Manager) HTTPHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.ServeChallenge(w, r) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Manager) ServeChallenge(w http.ResponseWriter, r *http.Request) bool {
	const p = "/.well-known/acme-challenge/"
	if r.URL == nil || !strings.HasPrefix(r.URL.Path, p) {
		return false
	}
	token := strings.TrimPrefix(r.URL.Path, p)
	m.mu.Lock()
	body, ok := m.tokens[token]
	m.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return true
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(body))
	return true
}

func (m *Manager) Maintain(ctx context.Context) {
	_ = m.IssueAll(ctx)
	t := time.NewTicker(12 * time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = m.IssueAll(ctx)
		}
	}
}

func (m *Manager) IssueAll(ctx context.Context) error {
	m.mu.Lock()
	if m.issuing {
		m.mu.Unlock()
		return nil
	}
	m.issuing = true
	names := m.names()
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.issuing = false
		m.mu.Unlock()
	}()
	var first error
	changed := false
	for _, name := range names {
		need, err := m.needsIssue(name)
		if err != nil && first == nil {
			first = err
		}
		if !need {
			continue
		}
		if err := m.issueOne(ctx, name); err != nil {
			log.Printf("acme %s: %v", name, err)
			m.mu.Lock()
			if m.certs[name] == nil {
				m.certs[name] = &hostCert{err: err.Error()}
			} else {
				m.certs[name].err = err.Error()
			}
			m.mu.Unlock()
			if first == nil {
				first = err
			}
			continue
		}
		changed = true
	}
	if changed {
		m.mu.Lock()
		m.publishPrimary()
		m.mu.Unlock()
		if m.onChange != nil {
			m.onChange()
		}
	}
	return first
}

func (m *Manager) needsIssue(name string) (bool, error) {
	m.mu.Lock()
	hc := m.certs[name]
	m.mu.Unlock()
	if hc == nil || hc.tls == nil {
		return true, nil
	}
	if hc.notAfter.IsZero() {
		return true, nil
	}
	if time.Until(hc.notAfter) < renewWindow {
		return true, nil
	}
	return false, nil
}

func (m *Manager) publishPrimary() {
	// Prefer public_host-shaped names (first in hosts()), else any issued cert.
	var pick *hostCert
	for _, h := range m.names() {
		if hc := m.certs[h]; hc != nil && hc.tls != nil && time.Now().Before(hc.notAfter) {
			pick = hc
			break
		}
	}
	if pick == nil {
		return
	}
	if err := writeTLSPair(m.certPath, m.keyPath, pick.tls); err != nil {
		log.Printf("acme write origin cert: %v", err)
	}
}

func writeTLSPair(certPath, keyPath string, c *tls.Certificate) error {
	if c == nil || len(c.Certificate) == 0 {
		return fmt.Errorf("empty cert")
	}
	var certPEM []byte
	for _, der := range c.Certificate {
		certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	var keyPEM []byte
	switch k := c.PrivateKey.(type) {
	case *ecdsa.PrivateKey:
		der, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return err
		}
		keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	default:
		der, err := x509.MarshalPKCS8PrivateKey(c.PrivateKey)
		if err != nil {
			return err
		}
		keyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	}
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return err
	}
	return os.WriteFile(keyPath, keyPEM, 0600)
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return host
}

// Eligible is true for names Let's Encrypt will consider (not IPs or docs placeholders).
func Eligible(host string) bool {
	host = normalizeHost(host)
	if host == "" || strings.ContainsAny(host, "/ \\") {
		return false
	}
	if net.ParseIP(host) != nil {
		return false
	}
	if !strings.Contains(host, ".") {
		return false
	}
	switch host {
	case "localhost", "vpn.example.com":
		return false
	}
	for _, suf := range []string{".local", ".lan", ".invalid", ".test", ".example", ".example.com", ".example.org", ".example.net"} {
		if strings.HasSuffix(host, suf) {
			return false
		}
	}
	return true
}
