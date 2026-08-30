package models

import (
	"testing"
	"time"
)

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

func TestXrayShareable(t *testing.T) {
	ok := Inbound{Protocol: ProtoVLESS, Transport: TransportTCP, Security: SecurityReality}
	if !ok.XrayShareable() {
		t.Fatal("vision")
	}
	grpc := Inbound{Protocol: ProtoVLESS, Transport: TransportGRPC, Security: SecurityReality}
	if !grpc.XrayShareable() {
		t.Fatal("reality grpc")
	}
	for _, in := range []Inbound{
		{Protocol: ProtoVLESS, Transport: TransportHTTPUpgrade, Security: SecurityReality},
		{Protocol: ProtoVLESS, Transport: TransportH2, Security: SecurityReality},
		{Protocol: ProtoVLESS, Transport: TransportXHTTP, Security: SecurityReality},
		{Protocol: ProtoVLESS, Transport: TransportH2, Security: SecurityTLS},
		{Protocol: ProtoVLESS, Transport: TransportXHTTP, Security: SecurityTLS},
		{Protocol: ProtoShadowTLS},
	} {
		if in.XrayShareable() {
			t.Fatalf("should be sing-box only: %+v", in)
		}
	}
	ws := Inbound{Protocol: ProtoVLESS, Transport: TransportWS, Security: SecurityTLS}
	if !ws.XrayShareable() {
		t.Fatal("ws")
	}
	hup := Inbound{Protocol: ProtoVLESS, Transport: TransportHTTPUpgrade, Security: SecurityTLS}
	if !hup.XrayShareable() {
		t.Fatal("httpupgrade tls")
	}
}

func TestRefreshPresence(t *testing.T) {
	now := time.Now()
	u := User{}
	u.RefreshPresence(now)
	if u.Online {
		t.Fatal("never seen")
	}
	old := now.Add(-2 * time.Minute)
	u.LastSeenAt = &old
	u.RefreshPresence(now)
	if u.Online {
		t.Fatal("stale should be offline")
	}
	fresh := now.Add(-10 * time.Second)
	u.LastSeenAt = &fresh
	u.RefreshPresence(now)
	if !u.Online {
		t.Fatal("recent should be online")
	}
}
