package models

import "testing"

func TestNormalizeRealityDest(t *testing.T) {
	if got := NormalizeRealityDest(""); got != DefaultRealityDest {
		t.Fatalf("empty %q", got)
	}
	if got := NormalizeRealityDest(" https://Gateway.iCloud.com/path "); got != "gateway.icloud.com" {
		t.Fatalf("got %q", got)
	}
	if err := ValidateRealityDest("www.microsoft.com", nil); err == nil {
		t.Fatal("microsoft dest must be rejected")
	}
	if err := ValidateRealityDest("gateway.icloud.com", []string{"vpn.example.com"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRealityDest("www.cloudflare.com", []string{"www.cloudflare.com"}); err == nil {
		t.Fatal("telegram dest should fail")
	}
	if err := ValidateRealityDest("1.2.3.4", nil); err == nil {
		t.Fatal("ip should fail")
	}
	st := Settings{RealityServerName: ""}
	if st.RealityDest() != DefaultRealityDest {
		t.Fatalf("settings fallback %q", st.RealityDest())
	}
}
