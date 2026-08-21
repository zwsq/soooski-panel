package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/zwsq/soooski-panel/internal/crypto"
	"github.com/zwsq/soooski-panel/internal/models"
)

func TestNormalizeAndValidateFakeDomain(t *testing.T) {
	if got := NormalizeFakeDomain(" https://WWW.Cloudflare.com/cdn "); got != "www.cloudflare.com" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeFakeDomain(""); got != DefaultFakeDomain {
		t.Fatalf("empty %q", got)
	}
	if err := ValidateFakeDomain("1.2.3.4", nil); err == nil {
		t.Fatal("ip should fail")
	}
	if err := ValidateFakeDomain("www.cloudflare.com", []string{"vpn.example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateFakeDomain("vpn.example.com", []string{"vpn.example.com", "www.microsoft.com"}); err == nil {
		t.Fatal("own domain should fail")
	}
	if err := ValidateFakeDomain("www.microsoft.com", []string{"www.microsoft.com"}); err == nil {
		t.Fatal("reality dest should fail")
	}
}

func TestRenderTOMLPerUser(t *testing.T) {
	domain := "www.cloudflare.com"
	alice := crypto.TelegramFakeTLSSecret(domain)
	bob := crypto.TelegramFakeTLSSecret(domain)
	raw := RenderTOML(models.Settings{
		PublicHost:         "1.2.3.4",
		TelegramFakeDomain: domain,
	}, []models.User{
		{ID: 2, Username: "alice", Enable: true, TelegramSecret: alice},
		{ID: 3, Username: "bob", Enable: true, TelegramSecret: bob},
		{ID: 4, Username: "off", Enable: false, TelegramSecret: crypto.TelegramFakeTLSSecret(domain)},
	})
	for _, part := range []string{
		`bind-to = "127.0.0.1:1001"`,
		`api-bind-to = "127.0.0.1:1002"`,
		`prefer-ip = "only-ipv4"`,
		`public-ipv4 = "1.2.3.4"`,
		`u2 = "` + alice + `"`,
		`u3 = "` + bob + `"`,
		`[secrets]`,
	} {
		if !strings.Contains(raw, part) {
			t.Fatalf("missing %s in %s", part, raw)
		}
	}
	if strings.Contains(raw, "u4") {
		t.Fatal("inactive user must not get a telegram secret")
	}
	if strings.Contains(raw, "\nsecret =") {
		t.Fatal("global secret must not be written")
	}
	if idx := strings.Index(raw, "[secrets]"); idx < strings.Index(raw, "bind-to") {
		t.Fatal("[secrets] must be last")
	}
}

func TestSecretName(t *testing.T) {
	if SecretName(12) != "u12" {
		t.Fatal(SecretName(12))
	}
	id, ok := ParseSecretName("u12")
	if !ok || id != 12 {
		t.Fatal(id, ok)
	}
	if _, ok := ParseSecretName("alice"); ok {
		t.Fatal("alice")
	}
}

func TestActiveExpirySkipsSecret(t *testing.T) {
	past := time.Now().Add(-time.Hour)
	raw := RenderTOML(models.Settings{TelegramFakeDomain: "www.cloudflare.com"}, []models.User{
		{ID: 1, Enable: true, ExpireAt: &past, TelegramSecret: crypto.TelegramFakeTLSSecret("www.cloudflare.com")},
	})
	if strings.Contains(raw, "u1 =") {
		t.Fatal("expired user must be omitted")
	}
}
