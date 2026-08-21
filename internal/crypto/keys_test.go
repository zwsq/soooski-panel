package crypto

import (
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestRealityAndWGKeys(t *testing.T) {
	priv, pub, err := RealityKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(priv); err != nil {
		t.Fatal(err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(pub); err != nil {
		t.Fatal(err)
	}
	if priv == pub {
		t.Fatal("keys equal")
	}
	wpriv, wpub, err := WireGuardKeyPair()
	if err != nil || wpriv == "" || wpub == "" {
		t.Fatal(wpriv, wpub, err)
	}
	hash, err := HashPassword("hi")
	if err != nil || !CheckPassword(hash, "hi") || CheckPassword(hash, "no") {
		t.Fatal(hash, err)
	}
}

func TestTelegramFakeTLSSecret(t *testing.T) {
	domain := "www.cloudflare.com"
	s := TelegramFakeTLSSecret(domain)
	if !strings.HasPrefix(s, "ee") {
		t.Fatalf("prefix %s", s)
	}
	want := hex.EncodeToString([]byte(domain))
	if !strings.HasSuffix(s, want) {
		t.Fatalf("domain hex %s vs %s", s, want)
	}
	if len(s) != 2+32+len(want) {
		t.Fatalf("len %d %s", len(s), s)
	}
	if !TelegramSecretMatchesDomain(s, domain) {
		t.Fatal("should match")
	}
	if TelegramSecretMatchesDomain(s, "www.microsoft.com") {
		t.Fatal("other domain")
	}
	a := TelegramFakeTLSSecret(domain)
	if a == s {
		t.Fatal("secrets should be random")
	}
}
