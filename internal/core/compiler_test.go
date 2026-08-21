package core

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/zwsq/soooski-panel/internal/core/catalog"
	"github.com/zwsq/soooski-panel/internal/models"
)

func sampleInput() CompileInput {
	var inbounds []models.Inbound
	for i, spec := range catalog.Defaults() {
		path := spec.Path
		if spec.NeedsRandomPath {
			path = "/p" + spec.Tag
		}
		inbounds = append(inbounds, models.Inbound{
			ID: int64(i + 1), Tag: spec.Tag, Protocol: spec.Protocol, Transport: spec.Transport,
			Security: spec.Security, Mode: spec.Mode, ListenPort: spec.ListenPort,
			InternalPort: spec.InternalPort, Path: path, Enable: true, Remark: spec.Remark,
		})
	}
	return CompileInput{
		Settings: models.Settings{
			PublicHost:        "vpn.example.com",
			RealityPrivateKey: "priv",
			RealityPublicKey:  "pub",
			RealityShortID:    "abcd1234",
			RealityServerName: "www.microsoft.com",
			SSPassword:        "c3NlcnZlcnBhc3N3b3JkMTI=",
			WGPrivateKey:      "wpriv",
			WGPublicKey:       "wpub",
			WGSubnet:          "10.66.66.0/24",
			ClashSecret:       "secret",
			TLSCertPath:       "/data/certs/server.crt",
			TLSKeyPath:        "/data/certs/server.key",
		},
		Users: []models.User{{
			Username: "alice", UUID: "11111111-1111-1111-1111-111111111111",
			Password: "trojanpass", SSPassword: "dXNlcjEyMzQ1Njc4OTA=", Enable: true,
			WGPublicKey: "alicepub", WGIP: "10.66.66.2",
		}},
		Inbounds: inbounds,
	}
}

func TestCompileIncludesDirectAndCDN(t *testing.T) {
	raw, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	rawIn, _ := json.Marshal(cfg["inbounds"])
	s := string(rawIn)
	for _, tag := range []string{
		"vless-reality", "hysteria2", "tuic", "shadowtls", "ss-inner",
		"vless-ws", "vless-grpc", "trojan-ws", "vmess-ws", "vless-httpupgrade",
		"vless-h2", "vless-xhttp", "vless-ws-direct", "vless-tcp", "vmess-h2", "trojan-h2",
	} {
		if !strings.Contains(s, `"tag":"`+tag+`"`) {
			t.Fatalf("missing inbound %s in %s", tag, s)
		}
	}
	if !strings.Contains(s, `"type":"http"`) {
		t.Fatal("missing HTTP/2 transport")
	}
	if strings.Contains(s, `"type":"wireguard"`) {
		t.Fatal("wireguard must not be an inbound in sing-box 1.11+")
	}
	rawEp, _ := json.Marshal(cfg["endpoints"])
	if !strings.Contains(string(rawEp), `"tag":"wireguard"`) {
		t.Fatalf("missing wireguard endpoint: %s", rawEp)
	}
	if !strings.Contains(s, `"listen":"127.0.0.1"`) {
		t.Fatal("cdn inbounds should bind localhost")
	}
	if !strings.Contains(s, "xtls-rprx-vision") {
		t.Fatal("missing vision flow")
	}
	if !strings.Contains(s, "reality") {
		t.Fatal("missing reality")
	}
	rawAll := string(raw)
	if !strings.Contains(rawAll, `"auth_user"`) || !strings.Contains(rawAll, "user-alice") {
		t.Fatalf("expected per-user outbound for traffic accounting:\n%s", rawAll)
	}
}

func TestDisabledUserOmitted(t *testing.T) {
	in := sampleInput()
	in.Users[0].Enable = false
	raw, err := Compile(in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "alice") {
		t.Fatal("disabled user should not appear")
	}
}

func TestCatalogCoversHiddifyMatrix(t *testing.T) {
	specs := catalog.Defaults()
	if len(specs) < 40 {
		t.Fatalf("got %d specs", len(specs))
	}
	var direct, cdn int
	for _, s := range specs {
		if s.Mode == models.ModeDirect {
			direct++
		}
		if s.Mode == models.ModeCDN {
			cdn++
		}
	}
	if direct < 20 || cdn < 16 {
		t.Fatalf("direct=%d cdn=%d", direct, cdn)
	}
}
