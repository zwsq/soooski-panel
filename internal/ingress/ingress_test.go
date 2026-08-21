package ingress

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zwsq/soooski-panel/internal/models"
)

func TestCamouflageAndPanelPassThrough(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	})
	ing := New(next, func() []Route {
		return RoutesFrom([]models.Inbound{{
			Enable: true, Mode: models.ModeCDN, Path: "/secret-ws", InternalPort: 20001, Transport: "ws",
		}})
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ing.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("camouflage %d", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !bytesContains(body, []byte("nginx")) {
		t.Fatalf("body %s", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/panel/", nil)
	rec = httptest.NewRecorder()
	ing.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("default admin prefix passthrough %d", rec.Code)
	}
}

func TestSecretAdminAndClientPaths(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Path", r.URL.Path)
		w.WriteHeader(204)
	})
	ing := New(next, func() []Route { return nil })
	ing.AdminPrefix = func() string { return "/aaa/bbb" }
	ing.ClientPrefix = func() string { return "/ccc" }

	req := httptest.NewRequest(http.MethodGet, "/aaa/bbb/api/login", nil)
	rec := httptest.NewRecorder()
	ing.ServeHTTP(rec, req)
	if rec.Code != 204 || rec.Header().Get("X-Path") != "/api/login" {
		t.Fatalf("admin rewrite %d %s", rec.Code, rec.Header().Get("X-Path"))
	}

	req = httptest.NewRequest(http.MethodGet, "/ccc/tok/clash", nil)
	rec = httptest.NewRecorder()
	ing.ServeHTTP(rec, req)
	if rec.Code != 204 || rec.Header().Get("X-Path") != "/sub/tok/clash" {
		t.Fatalf("client rewrite %d %s", rec.Code, rec.Header().Get("X-Path"))
	}

	req = httptest.NewRequest(http.MethodGet, "/api/login", nil)
	rec = httptest.NewRecorder()
	ing.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("api at root should be camouflage, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/favicon.png", nil)
	rec = httptest.NewRecorder()
	ing.ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("brand asset should reach the panel file server, got %d", rec.Code)
	}
}

func TestOursSNI(t *testing.T) {
	ing := New(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), func() []Route { return nil })
	ing.HostsFn = func() []string { return []string{"vpn.example.com", "cdn.my-domain.com"} }
	if !ing.Ours("") {
		t.Fatal("empty SNI should hit the panel, not REALITY")
	}
	if !ing.Ours("1.2.3.4") {
		t.Fatal("IP SNI should hit the panel")
	}
	if !ing.Ours("vpn.example.com") {
		t.Fatal("configured host should hit the panel")
	}
	if ing.Ours("www.microsoft.com") {
		t.Fatal("REALITY dest SNI must be forwarded to REALITY")
	}
}

func TestTelegramSNI(t *testing.T) {
	ing := New(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), func() []Route { return nil })
	ing.HostsFn = func() []string { return []string{"vpn.example.com"} }
	ing.RealityAddr = "127.0.0.1:12001"
	ing.TelegramFn = func() (string, string, bool) {
		return "www.cloudflare.com", "127.0.0.1:1001", true
	}
	if ing.RouteSNI("vpn.example.com") != LocalHTTPS {
		t.Fatal("panel host must stay on HTTPS")
	}
	if ing.RouteSNI("www.cloudflare.com") != "127.0.0.1:1001" {
		t.Fatalf("telegram SNI: %s", ing.RouteSNI("www.cloudflare.com"))
	}
	if ing.RouteSNI("www.microsoft.com") != "127.0.0.1:12001" {
		t.Fatalf("reality SNI: %s", ing.RouteSNI("www.microsoft.com"))
	}
	ing.TelegramFn = func() (string, string, bool) { return "www.cloudflare.com", "127.0.0.1:1001", false }
	if ing.RouteSNI("www.cloudflare.com") != "127.0.0.1:12001" {
		t.Fatal("disabled telegram must fall through to REALITY")
	}
}

func TestRoutesFromDirectAndH2(t *testing.T) {
	routes := RoutesFrom([]models.Inbound{
		{Enable: true, Mode: models.ModeDirect, Path: "/direct-ws", InternalPort: 20020, Transport: "ws"},
		{Enable: true, Mode: models.ModeCDN, Path: "/h2", InternalPort: 20010, Transport: models.TransportH2},
		{Enable: true, Mode: models.ModeDirect, Path: "", InternalPort: 8447, Transport: "tcp", Security: models.SecurityTLS, ListenPort: 8447},
	})
	if len(routes) != 2 {
		t.Fatalf("got %#v", routes)
	}
	if routes[0].H2C || routes[0].Addr != "127.0.0.1:20020" {
		t.Fatalf("ws %#v", routes[0])
	}
	if !routes[1].H2C {
		t.Fatalf("h2 should use h2c proxy %#v", routes[1])
	}
}

func bytesContains(b, sub []byte) bool {
	return len(b) >= len(sub) && (string(b) != "" && (func() bool {
		s := string(b)
		p := string(sub)
		for i := 0; i+len(p) <= len(s); i++ {
			if s[i:i+len(p)] == p {
				return true
			}
		}
		return false
	})())
}
