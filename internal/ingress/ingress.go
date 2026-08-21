package ingress

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/zwsq/soooski-panel/internal/models"
	"golang.org/x/net/http2"
)

type Route struct {
	Path string
	Addr string
	H2C  bool // gRPC and HTTP/2 (h2 / xHTTP) — proxy with h2c
}

type Ingress struct {
	Next         http.Handler
	RoutesFn     func() []Route
	AdminPrefix  func() string
	ClientPrefix func() string
	HostsFn      func() []string
	Challenge    func(http.ResponseWriter, *http.Request) bool
	RealityAddr  string
	// TelegramFn returns FakeTLS SNI, local mtg address, and whether the proxy is on.
	TelegramFn func() (fakeDomain, addr string, ok bool)
	RealIP     bool
	mu         sync.Mutex
	httpProxy  map[string]*httputil.ReverseProxy
	h2cProxy   map[string]*httputil.ReverseProxy
}

func New(next http.Handler, routes func() []Route) *Ingress {
	return &Ingress{
		Next: next, RoutesFn: routes, RealIP: true,
		httpProxy:    map[string]*httputil.ReverseProxy{},
		h2cProxy:     map[string]*httputil.ReverseProxy{},
		AdminPrefix:  func() string { return "/panel" },
		ClientPrefix: func() string { return "/sub" },
	}
}

func RoutesFrom(inbounds []models.Inbound) []Route {
	var out []Route
	for _, in := range inbounds {
		if !in.PathMuxed() {
			continue
		}
		path := in.Path
		if !strings.HasPrefix(path, "/") && in.Transport != models.TransportGRPC {
			path = "/" + path
		}
		out = append(out, Route{
			Path: path,
			Addr: "127.0.0.1:" + itoa(in.InternalPort),
			H2C:  in.H2C(),
		})
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func (i *Ingress) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if i.Challenge != nil && i.Challenge(w, r) {
		return
	}
	if i.RealIP {
		if ip, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && isCloudflare(ip) {
			if v := r.Header.Get("CF-Connecting-IP"); v != "" {
				r.RemoteAddr = net.JoinHostPort(v, "0")
				r.Header.Set("X-Real-IP", v)
			}
		}
	}
	path := r.URL.Path

	for _, rt := range i.RoutesFn() {
		p := rt.Path
		if rt.H2C {
			if path == "/"+strings.TrimPrefix(p, "/") || strings.HasPrefix(path, "/"+strings.TrimPrefix(p, "/")+"/") {
				i.proxy(w, r, rt)
				return
			}
			continue
		}
		if path == p || strings.HasPrefix(path, p+"/") {
			i.proxy(w, r, rt)
			return
		}
	}

	if prefix := ""; i.ClientPrefix != nil {
		prefix = strings.TrimRight(i.ClientPrefix(), "/")
		if prefix != "" && prefix != "/" && (path == prefix || strings.HasPrefix(path, prefix+"/")) {
			rest := strings.TrimPrefix(path, prefix)
			if rest == "" {
				rest = "/"
			}
			r.URL.Path = "/sub" + rest
			i.Next.ServeHTTP(w, r)
			return
		}
	}
	if i.AdminPrefix != nil {
		prefix := strings.TrimRight(i.AdminPrefix(), "/")
		if prefix != "" && prefix != "/" && (path == prefix || strings.HasPrefix(path, prefix+"/")) {
			rest := strings.TrimPrefix(path, prefix)
			if rest == "" {
				rest = "/"
			}
			r.URL.Path = rest
			i.Next.ServeHTTP(w, r)
			return
		}
	}
	if brandAsset(path) {
		if path == "/favicon.ico" {
			r.URL.Path = "/favicon.png"
		}
		i.Next.ServeHTTP(w, r)
		return
	}
	camouflage(w, r)
}

func (i *Ingress) proxy(w http.ResponseWriter, r *http.Request, rt Route) {
	p := i.getProxy(rt)
	p.ServeHTTP(w, r)
}

func (i *Ingress) getProxy(rt Route) *httputil.ReverseProxy {
	i.mu.Lock()
	defer i.mu.Unlock()
	store := i.httpProxy
	if rt.H2C {
		store = i.h2cProxy
	}
	if p, ok := store[rt.Addr]; ok {
		return p
	}
	target, _ := url.Parse("http://" + rt.Addr)
	p := httputil.NewSingleHostReverseProxy(target)
	orig := p.Director
	p.Director = func(req *http.Request) {
		host := req.Host
		orig(req)
		if host != "" {
			req.Host = host
			req.Header.Set("Host", host)
		}
	}
	if rt.H2C {
		p.Transport = &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
				var d net.Dialer
				d.Timeout = 5 * time.Second
				return d.DialContext(ctx, network, addr)
			},
		}
	} else {
		p.FlushInterval = -1
	}
	p.ErrorLog = log.New(io.Discard, "", 0)
	store[rt.Addr] = p
	return p
}

func brandAsset(path string) bool {
	switch path {
	case "/favicon.png", "/favicon.ico", "/apple-touch-icon.png", "/logo.png":
		return true
	default:
		return false
	}
}

func camouflage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Server", "nginx")
	_, _ = w.Write([]byte(camouflageHTML))
}

const camouflageHTML = `<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto; font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>
<p><em>Thank you for using nginx.</em></p>
</body>
</html>
`
