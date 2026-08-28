package core

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/zwsq/soooski-panel/internal/models"
)

func TestVLESSRealityLink(t *testing.T) {
	in := sampleInput()
	links := UserLinks(in.Users[0], in.Settings, nil, in.Inbounds)
	var reality string
	var ws string
	for _, l := range links {
		if l.Tag == "vless-reality" {
			reality = l.URI
		}
		if l.Tag == "vless-ws" {
			ws = l.URI
		}
		if l.Tag == "vless-ws-http" && (!strings.Contains(l.URI, "security=tls") || !strings.Contains(l.URI, ":443")) {
			t.Fatalf("cdn http origin must still be advertised as tls:443 to the edge, got %s", l.URI)
		}
	}
	if !strings.HasPrefix(reality, "vless://") {
		t.Fatalf("reality %s", reality)
	}
	for _, part := range []string{"security=reality", "pbk=pub", "sid=abcd1234", "flow=xtls-rprx-vision", "headerType=none", ":443"} {
		if !strings.Contains(reality, part) {
			t.Fatalf("reality missing %s: %s", part, reality)
		}
	}
	if !strings.Contains(ws, "type=ws") && !strings.Contains(ws, "type%3Dws") {
		// query is encoded
		if !strings.Contains(ws, "ws") {
			t.Fatalf("ws link %s", ws)
		}
	}
	if !strings.Contains(ws, ":443") {
		t.Fatalf("cdn should advertise 443, got %s", ws)
	}
	if strings.Contains(ws, "h2") {
		t.Fatalf("cdn websocket must not advertise h2 ALPN: %s", ws)
	}
	if !strings.Contains(ws, "http") {
		t.Fatalf("cdn websocket should pin http/1.1 alpn: %s", ws)
	}
}

func TestH2AndDirectMuxLinks(t *testing.T) {
	in := sampleInput()
	domains := []models.Domain{
		{Domain: "direct.example.com", Mode: models.ModeDirect, Enable: true},
		{Domain: "cdn.example.com", Mode: models.ModeCDN, Enable: true, Provider: "cloudflare"},
	}
	links := UserLinks(in.Users[0], in.Settings, domains, in.Inbounds)
	var h2, xhttp, directWS, realityH2 string
	for _, l := range links {
		switch l.Tag {
		case "vless-h2":
			h2 = l.URI
		case "vless-xhttp":
			xhttp = l.URI
		case "vless-ws-direct":
			directWS = l.URI
		case "vless-reality-h2":
			realityH2 = l.URI
		}
	}
	if !strings.Contains(h2, "cdn.example.com") || !strings.Contains(h2, "alpn=h2") {
		t.Fatalf("cdn h2: %s", h2)
	}
	if !strings.Contains(h2, "type=http") && !strings.Contains(h2, "type%3Dhttp") {
		t.Fatalf("h2 should advertise type=http: %s", h2)
	}
	if !strings.Contains(xhttp, "type=http") && !strings.Contains(xhttp, "type%3Dhttp") {
		t.Fatalf("xhttp stand-in should advertise type=http: %s", xhttp)
	}
	if !strings.Contains(directWS, "direct.example.com") || strings.Contains(directWS, "cdn.example.com") {
		t.Fatalf("direct ws host: %s", directWS)
	}
	if !strings.Contains(directWS, ":443") {
		t.Fatalf("direct path mux should be 443: %s", directWS)
	}
	if strings.Contains(directWS, "alpn=h2") {
		t.Fatalf("direct websocket must not advertise h2: %s", directWS)
	}
	if !strings.Contains(realityH2, "security=reality") {
		t.Fatalf("reality h2: %s", realityH2)
	}
	y := ClashYAML(in.Users[0], in.Settings, domains, in.Inbounds)
	if !strings.Contains(y, "network: h2") {
		t.Fatalf("clash missing h2:\n%s", y)
	}
}

func TestClashAndSingBox(t *testing.T) {
	in := sampleInput()
	u := in.Users[0]
	y := ClashYAML(u, in.Settings, nil, in.Inbounds)
	if !strings.Contains(y, "type: hysteria2") || !strings.Contains(y, "reality-opts") {
		t.Fatalf("clash yaml incomplete:\n%s", y)
	}
	raw, err := SingBoxClient(u, in.Settings, nil, in.Inbounds)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"type": "selector"`) {
		t.Fatalf("sing-box client %s", raw)
	}
	sub := V2RaySubscription(UserLinks(u, in.Settings, nil, in.Inbounds))
	if sub == "" {
		t.Fatal("empty sub")
	}
}

func TestDomainModePicksHost(t *testing.T) {
	in := sampleInput()
	domains := []models.Domain{
		{Domain: "direct.example.com", Mode: models.ModeDirect, Enable: true},
		{Domain: "cdn.example.com", Mode: models.ModeCDN, Enable: true, Provider: "cloudflare"},
	}
	links := UserLinks(in.Users[0], in.Settings, domains, in.Inbounds)
	var reality, ws string
	for _, l := range links {
		if l.Tag == "vless-reality" {
			reality = l.URI
		}
		if l.Tag == "vless-ws" {
			ws = l.URI
		}
	}
	if !strings.Contains(reality, "direct.example.com") {
		t.Fatalf("direct host: %s", reality)
	}
	if !strings.Contains(ws, "cdn.example.com") {
		t.Fatalf("cdn host: %s", ws)
	}
}

func TestTelegramLinks(t *testing.T) {
	in := sampleInput()
	in.Settings.TelegramEnabled = true
	in.Settings.TelegramFakeDomain = "www.cloudflare.com"
	in.Users[0].TelegramSecret = "ee" + strings.Repeat("ab", 16) + "7777"
	links := UserLinks(in.Users[0], in.Settings, nil, in.Inbounds)
	var tg, https string
	for _, l := range links {
		if l.Tag == "telegram" {
			tg = l.URI
		}
		if l.Tag == "telegram-https" {
			https = l.URI
		}
	}
	if !strings.HasPrefix(tg, "tg://proxy?") || !strings.Contains(tg, "server=vpn.example.com") || !strings.Contains(tg, "port=443") || !strings.Contains(tg, "secret=eeabab") {
		t.Fatalf("tg link %s", tg)
	}
	if !strings.HasPrefix(https, "https://t.me/proxy?") {
		t.Fatalf("https link %s", https)
	}
	decoded, err := base64.StdEncoding.DecodeString(V2RaySubscription(links))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(decoded), "tg://") || strings.Contains(string(decoded), "t.me/proxy") {
		t.Fatalf("v2ray sub should skip telegram: %s", decoded)
	}
	off := sampleInput()
	if n := telegramProxyLinks(off.Users[0], off.Settings); len(n) != 0 {
		t.Fatalf("disabled: %#v", n)
	}
	other := in.Users[0]
	other.TelegramSecret = "ee" + strings.Repeat("cd", 16) + "7777"
	a := telegramProxyLinks(in.Users[0], in.Settings)
	b := telegramProxyLinks(other, in.Settings)
	if a[0].URI == b[0].URI {
		t.Fatal("users must have different telegram secrets")
	}
}
