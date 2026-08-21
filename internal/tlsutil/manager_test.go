package tlsutil

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/zwsq/soooski-panel/internal/crypto"
)

func TestEligible(t *testing.T) {
	ok := []string{"vpn.my-domain.com", "cdn.example.co", "panel.ir"}
	for _, h := range ok {
		if !Eligible(h) {
			t.Fatalf("expected eligible: %s", h)
		}
	}
	no := []string{
		"", "localhost", "vpn.example.com", "10.0.0.1", "::1",
		"foo.local", "bar.test", "x.example.com", "no-dot",
	}
	for _, h := range no {
		if Eligible(h) {
			t.Fatalf("expected ineligible: %s", h)
		}
	}
}

func TestChallengeAndStatus(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "server.crt")
	key := filepath.Join(dir, "server.key")
	if err := crypto.EnsureSelfSigned(cert, key, "localhost"); err != nil {
		t.Fatal(err)
	}
	m, err := New("a@b.c", filepath.Join(dir, "acme"), cert, key, func() []string { return []string{"vpn.example.com", "panel.ir"} })
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	m.tokens["abc"] = "token-body"
	m.mu.Unlock()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/.well-known/acme-challenge/abc", nil)
	if !m.ServeChallenge(rec, req) || rec.Body.String() != "token-body" {
		t.Fatalf("challenge %d %s", rec.Code, rec.Body.String())
	}
	st := m.Status()
	var sawIneligible, sawMissing bool
	for _, s := range st {
		if s.Domain == "vpn.example.com" && s.State == "ineligible" {
			sawIneligible = true
		}
		if s.Domain == "panel.ir" && (s.State == "missing" || s.State == "pending") {
			sawMissing = true
		}
	}
	if !sawIneligible || !sawMissing {
		t.Fatalf("status %+v", st)
	}
}
