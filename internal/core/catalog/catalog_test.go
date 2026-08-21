package catalog

import (
	"testing"

	"github.com/zwsq/soooski-panel/internal/models"
)

func TestDefaultsUniqueAndCoverH2(t *testing.T) {
	specs := Defaults()
	if len(specs) < 40 {
		t.Fatalf("got %d specs, want a full CDN+direct matrix", len(specs))
	}
	tags := map[string]struct{}{}
	internal := map[int]string{}
	publicTCP := map[int]string{}
	var direct, cdn int
	var sawH2, sawXHTTP, sawDirectWS, sawCDNH2 bool
	for _, s := range specs {
		if _, ok := tags[s.Tag]; ok {
			t.Fatalf("duplicate tag %s", s.Tag)
		}
		tags[s.Tag] = struct{}{}
		if s.InternalPort > 0 {
			if prev, ok := internal[s.InternalPort]; ok {
				t.Fatalf("internal port %d used by %s and %s", s.InternalPort, prev, s.Tag)
			}
			internal[s.InternalPort] = s.Tag
		}
		if s.ListenPort > 0 && s.ListenPort != 443 && s.ListenPort != 80 &&
			s.Transport != models.TransportUDP && s.Protocol != models.ProtoShadowTLS {
			if s.InternalPort == 0 || s.InternalPort == s.ListenPort {
				if prev, ok := publicTCP[s.ListenPort]; ok {
					t.Fatalf("public TCP %d used by %s and %s", s.ListenPort, prev, s.Tag)
				}
				publicTCP[s.ListenPort] = s.Tag
			}
		}
		if s.Mode == models.ModeDirect {
			direct++
		} else {
			cdn++
		}
		if s.Tag == "vless-h2" {
			sawCDNH2 = true
		}
		if s.Tag == "vless-ws-direct" {
			sawDirectWS = true
		}
		if s.Transport == models.TransportH2 {
			sawH2 = true
		}
		if s.Transport == models.TransportXHTTP {
			sawXHTTP = true
		}
	}
	if !sawH2 || !sawXHTTP || !sawDirectWS || !sawCDNH2 {
		t.Fatalf("missing protocol coverage h2=%v xhttp=%v direct-ws=%v cdn-h2=%v", sawH2, sawXHTTP, sawDirectWS, sawCDNH2)
	}
	if direct < 20 || cdn < 16 {
		t.Fatalf("direct=%d cdn=%d", direct, cdn)
	}
	for _, tag := range []string{"vless-h2", "vless-xhttp", "vless-ws-direct", "vless-reality-h2", "vmess-httpupgrade", "trojan-h2"} {
		if _, ok := tags[tag]; !ok {
			t.Fatalf("missing %s", tag)
		}
	}
}
