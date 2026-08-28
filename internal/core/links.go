package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/zwsq/soooski-panel/internal/crypto"
	"github.com/zwsq/soooski-panel/internal/models"
)

type Link struct {
	Name     string `json:"name"`
	URI      string `json:"uri"`
	Tag      string `json:"tag"`
	Mode     string `json:"mode"`
	Protocol string `json:"protocol"`
	Xray     bool   `json:"xray"`
}

func hostFor(st models.Settings, domains []models.Domain, mode string) (host string, sni string) {
	host = strings.TrimSpace(st.PublicHost)
	for _, d := range domains {
		if !d.Enable {
			continue
		}
		if d.Mode == mode {
			return d.Domain, d.Domain
		}
	}
	for _, d := range domains {
		if d.Enable {
			if host == "" {
				host = d.Domain
			}
			break
		}
	}
	if host == "" {
		host = "127.0.0.1"
	}
	sni = host
	return host, sni
}

func UserLinks(u models.User, st models.Settings, domains []models.Domain, inbounds []models.Inbound) []Link {
	var out []Link
	for _, spec := range inbounds {
		if !spec.Enable {
			continue
		}
		host, sni := hostFor(st, domains, spec.Mode)
		name := fmt.Sprintf("%s | %s", u.Username, spec.Tag)
		xray := spec.XrayShareable()
		switch spec.Protocol {
		case models.ProtoVLESS:
			out = append(out, Link{Name: name, Tag: spec.Tag, Mode: spec.Mode, Protocol: spec.Protocol, URI: vlessURI(u, spec, st, host, sni), Xray: xray})
		case models.ProtoVMess:
			out = append(out, Link{Name: name, Tag: spec.Tag, Mode: spec.Mode, Protocol: spec.Protocol, URI: vmessURI(u, spec, st, host, sni), Xray: xray})
		case models.ProtoTrojan:
			out = append(out, Link{Name: name, Tag: spec.Tag, Mode: spec.Mode, Protocol: spec.Protocol, URI: trojanURI(u, spec, st, host, sni), Xray: xray})
		case models.ProtoHysteria2:
			out = append(out, Link{Name: name, Tag: spec.Tag, Mode: spec.Mode, Protocol: spec.Protocol, URI: hy2URI(u, spec, st, host, sni), Xray: xray})
		case models.ProtoTUIC:
			out = append(out, Link{Name: name, Tag: spec.Tag, Mode: spec.Mode, Protocol: spec.Protocol, URI: tuicURI(u, spec, st, host, sni), Xray: xray})
		case models.ProtoShadowTLS:
			out = append(out, Link{Name: name, Tag: spec.Tag, Mode: spec.Mode, Protocol: spec.Protocol, URI: ssURI(u, spec, st, host), Xray: xray})
		case models.ProtoWireGuard:
			out = append(out, Link{Name: name, Tag: spec.Tag, Mode: spec.Mode, Protocol: spec.Protocol, URI: wgURI(u, spec, st, host), Xray: xray})
		}
	}
	out = append(out, telegramProxyLinks(u, st)...)
	return out
}

func telegramProxyLinks(u models.User, st models.Settings) []Link {
	if !st.TelegramEnabled || strings.TrimSpace(u.TelegramSecret) == "" {
		return nil
	}
	host := strings.TrimSpace(st.PublicHost)
	if host == "" {
		host = "127.0.0.1"
	}
	q := url.Values{}
	q.Set("server", host)
	q.Set("port", "443")
	q.Set("secret", u.TelegramSecret)
	qs := q.Encode()
	return []Link{
		{Name: "Telegram MTProto", URI: "tg://proxy?" + qs, Tag: "telegram", Mode: models.ModeDirect, Protocol: "mtproto"},
		{Name: "Telegram MTProto", URI: "https://t.me/proxy?" + qs, Tag: "telegram-https", Mode: models.ModeDirect, Protocol: "mtproto"},
	}
}

func linkALPN(spec models.Inbound) string {
	if spec.Transport == models.TransportGRPC || models.HTTP2Transport(spec.Transport) {
		return "h2"
	}
	// v2rayNG / Xray WebSocket over HTTP/2 fails. Chrome fingerprint would
	// otherwise negotiate h2 and the path would never upgrade.
	return "http/1.1"
}

func tlsInsecure(st models.Settings, spec models.Inbound, host string) bool {
	if spec.Mode == models.ModeCDN {
		return false
	}
	return !crypto.PublicCertTrusts(st.TLSCertPath, host)
}

func vlessURI(u models.User, spec models.Inbound, st models.Settings, host, sni string) string {
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("type", spec.Transport)
	if spec.Transport == models.TransportTCP {
		q.Set("type", "tcp")
	}
	port := spec.ListenPort
	switch {
	case spec.Security == models.SecurityReality:
		q.Set("security", "reality")
		q.Set("sni", st.RealityDest())
		q.Set("fp", "chrome")
		q.Set("pbk", st.RealityPublicKey)
		q.Set("sid", st.RealityShortID)
		if spec.Transport == models.TransportTCP {
			q.Set("flow", "xtls-rprx-vision")
			q.Set("headerType", "none")
		}
		setPathQuery(q, spec, st.RealityDest())
	case spec.Mode == models.ModeCDN:
		q.Set("security", "tls")
		q.Set("sni", sni)
		q.Set("fp", "chrome")
		q.Set("alpn", linkALPN(spec))
		port = 443
		setPathQuery(q, spec, sni)
	case spec.Security == models.SecurityTLS:
		q.Set("security", "tls")
		q.Set("sni", sni)
		q.Set("fp", "chrome")
		q.Set("alpn", linkALPN(spec))
		if tlsInsecure(st, spec, sni) {
			q.Set("allowInsecure", "1")
		}
		setPathQuery(q, spec, sni)
	default:
		q.Set("security", "none")
		port = 80
		setPathQuery(q, spec, sni)
	}
	if port == 0 {
		port = 443
	}
	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", u.UUID, host, port, q.Encode(), url.QueryEscape(u.Username+"-"+spec.Tag))
}

func setPathQuery(q url.Values, spec models.Inbound, host string) {
	path := spec.Path
	if path == "" {
		path = "/" + spec.Tag
	}
	switch spec.Transport {
	case models.TransportWS, models.TransportHTTPUpgrade:
		q.Set("path", path)
		q.Set("host", host)
		if spec.Transport == models.TransportHTTPUpgrade {
			q.Set("type", "httpupgrade")
		} else {
			q.Set("type", "ws")
		}
	case models.TransportGRPC:
		q.Set("serviceName", strings.TrimPrefix(path, "/"))
		q.Set("mode", "gun")
		q.Set("type", "grpc")
	case models.TransportH2, models.TransportXHTTP:
		q.Set("path", path)
		q.Set("host", host)
		q.Set("type", "http")
	}
}

func vmessURI(u models.User, spec models.Inbound, st models.Settings, host, sni string) string {
	port := spec.ListenPort
	tls := "none"
	net := spec.Transport
	path := spec.Path
	if spec.Mode == models.ModeCDN {
		tls = "tls"
		port = 443
	} else if spec.Security == models.SecurityTLS {
		tls = "tls"
	}
	if port == 0 {
		port = 443
	}
	if net == models.TransportTCP {
		net = "tcp"
	}
	obj := map[string]any{
		"v":    "2",
		"ps":   u.Username + "-" + spec.Tag,
		"add":  host,
		"port": strconv.Itoa(port),
		"id":   u.UUID,
		"aid":  "0",
		"scy":  "auto",
		"net":  net,
		"type": "none",
		"host": sni,
		"path": path,
		"tls":  tls,
		"sni":  sni,
		"fp":   "chrome",
		"alpn": linkALPN(spec),
	}
	if spec.Transport == models.TransportGRPC {
		obj["net"] = "grpc"
		obj["path"] = strings.TrimPrefix(path, "/")
		obj["type"] = "gun"
	}
	if models.HTTP2Transport(spec.Transport) {
		obj["net"] = "http"
		obj["type"] = "none"
		obj["path"] = path
	}
	if tlsInsecure(st, spec, sni) {
		obj["skip-cert-verify"] = true
		obj["allowInsecure"] = 1
	}
	raw, _ := json.Marshal(obj)
	return "vmess://" + base64.StdEncoding.EncodeToString(raw)
}

func trojanURI(u models.User, spec models.Inbound, st models.Settings, host, sni string) string {
	q := url.Values{}
	q.Set("security", "tls")
	q.Set("sni", sni)
	q.Set("fp", "chrome")
	q.Set("type", spec.Transport)
	port := spec.ListenPort
	if spec.Mode == models.ModeCDN {
		port = 443
		q.Set("alpn", linkALPN(spec))
		setPathQuery(q, spec, sni)
	} else if spec.Security == models.SecurityTLS {
		q.Set("alpn", linkALPN(spec))
		if tlsInsecure(st, spec, sni) {
			q.Set("allowInsecure", "1")
		}
		if spec.Transport == models.TransportTCP {
			q.Set("type", "tcp")
		} else {
			setPathQuery(q, spec, sni)
		}
	} else {
		q.Set("security", "none")
		port = 80
		setPathQuery(q, spec, sni)
	}
	if port == 0 {
		port = 443
	}
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", url.QueryEscape(u.Password), host, port, q.Encode(), url.QueryEscape(u.Username+"-"+spec.Tag))
}

func hy2URI(u models.User, spec models.Inbound, st models.Settings, host, sni string) string {
	q := url.Values{}
	q.Set("sni", sni)
	if tlsInsecure(st, spec, sni) {
		q.Set("insecure", "1")
	}
	if st.HY2Obfs != "" {
		q.Set("obfs", "salamander")
		q.Set("obfs-password", st.HY2Obfs)
	}
	port := spec.ListenPort
	if port == 0 {
		port = 443
	}
	return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s", url.QueryEscape(u.Password), host, port, q.Encode(), url.QueryEscape(u.Username+"-hy2"))
}

func tuicURI(u models.User, spec models.Inbound, st models.Settings, host, sni string) string {
	q := url.Values{}
	q.Set("sni", sni)
	q.Set("congestion_control", "bbr")
	q.Set("udp_relay_mode", "native")
	if tlsInsecure(st, spec, sni) {
		q.Set("insecure", "1")
	}
	port := spec.ListenPort
	if port == 0 {
		port = 8448
	}
	return fmt.Sprintf("tuic://%s:%s@%s:%d?%s#%s", u.UUID, url.QueryEscape(u.Password), host, port, q.Encode(), url.QueryEscape(u.Username+"-tuic"))
}

func ssURI(u models.User, spec models.Inbound, st models.Settings, host string) string {
	userInfo := base64.StdEncoding.EncodeToString([]byte("2022-blake3-aes-128-gcm:" + st.SSPassword + ":" + u.SSPassword))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, host, spec.ListenPort, url.QueryEscape(u.Username+"-ss"))
}

func wgURI(u models.User, spec models.Inbound, st models.Settings, host string) string {
	return fmt.Sprintf("wireguard://%s@%s:%d?publickey=%s&address=%s/32#%s",
		url.QueryEscape(u.WGPrivateKey), host, spec.ListenPort,
		url.QueryEscape(st.WGPublicKey), u.WGIP, url.QueryEscape(u.Username+"-wg"))
}

// V2RaySubscription is the default for v2rayN / v2rayNG (Xray-core).
// Xray 26 cannot load ShadowTLS, REALITY+HTTPUpgrade, or sing-box HTTP/2 /
// xHTTP stand-ins (type=http is HTTP/1.1 camouflage, not H2). Those stay in
// Clash YAML and sing-box JSON.
func V2RaySubscription(links []Link) string {
	var b strings.Builder
	n := 0
	for _, l := range links {
		if l.Protocol == "mtproto" || strings.HasPrefix(l.URI, "tg://") || strings.Contains(l.URI, "t.me/proxy") {
			continue
		}
		if !l.Xray {
			continue
		}
		if n > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l.URI)
		n++
	}
	return base64.StdEncoding.EncodeToString([]byte(b.String()))
}

func ClashYAML(u models.User, st models.Settings, domains []models.Domain, inbounds []models.Inbound) string {
	var b strings.Builder
	b.WriteString("mixed-port: 7890\nallow-lan: false\nmode: rule\nlog-level: info\n")
	b.WriteString("proxies:\n")
	var names []string
	for _, spec := range inbounds {
		if !spec.Enable {
			continue
		}
		name := sanitizeName(u.Username + "-" + spec.Tag)
		block := clashProxy(name, u, spec, st, domains)
		if block == "" {
			continue
		}
		names = append(names, name)
		b.WriteString(block)
	}
	b.WriteString("proxy-groups:\n")
	b.WriteString("  - name: PROXY\n    type: select\n    proxies:\n")
	if len(names) == 0 {
		b.WriteString("      - DIRECT\n")
	}
	for _, n := range names {
		fmt.Fprintf(&b, "      - %s\n", n)
	}
	b.WriteString("rules:\n  - MATCH,PROXY\n")
	return b.String()
}

func clashProxy(name string, u models.User, spec models.Inbound, st models.Settings, domains []models.Domain) string {
	host, sni := hostFor(st, domains, spec.Mode)
	port := spec.ListenPort
	if spec.Mode == models.ModeCDN {
		port = 443
	}
	if port == 0 {
		port = 443
	}
	insecure := tlsInsecure(st, spec, sni)
	skip := ""
	if insecure {
		skip = "    skip-cert-verify: true\n"
	}
	switch spec.Protocol {
	case models.ProtoVLESS:
		var b strings.Builder
		fmt.Fprintf(&b, "  - name: %q\n    type: vless\n    server: %s\n    port: %d\n    uuid: %s\n    udp: true\n    packet-encoding: xudp\n", name, host, port, u.UUID)
		if spec.Security == models.SecurityReality {
			b.WriteString("    tls: true\n    client-fingerprint: chrome\n")
			if spec.Transport == models.TransportTCP {
				b.WriteString("    flow: xtls-rprx-vision\n")
			}
			fmt.Fprintf(&b, "    servername: %s\n    reality-opts:\n      public-key: %s\n      short-id: %s\n", st.RealityDest(), st.RealityPublicKey, st.RealityShortID)
			clashTransport(&b, spec, st.RealityDest())
		} else if spec.Mode == models.ModeCDN || spec.Security == models.SecurityTLS {
			fmt.Fprintf(&b, "    tls: true\n    client-fingerprint: chrome\n    servername: %s\n    alpn:\n      - %s\n%s", sni, linkALPN(spec), skip)
			clashTransport(&b, spec, sni)
		} else {
			clashTransport(&b, spec, sni)
		}
		return b.String()
	case models.ProtoVMess:
		var b strings.Builder
		fmt.Fprintf(&b, "  - name: %q\n    type: vmess\n    server: %s\n    port: %d\n    uuid: %s\n    alterId: 0\n    cipher: auto\n    udp: true\n", name, host, port, u.UUID)
		if spec.Mode == models.ModeCDN || spec.Security == models.SecurityTLS {
			fmt.Fprintf(&b, "    tls: true\n    servername: %s\n    client-fingerprint: chrome\n%s", sni, skip)
		}
		clashTransport(&b, spec, sni)
		return b.String()
	case models.ProtoTrojan:
		var b strings.Builder
		fmt.Fprintf(&b, "  - name: %q\n    type: trojan\n    server: %s\n    port: %d\n    password: %s\n    udp: true\n    sni: %s\n    client-fingerprint: chrome\n%s", name, host, port, u.Password, sni, skip)
		clashTransport(&b, spec, sni)
		return b.String()
	case models.ProtoHysteria2:
		obfs := ""
		if st.HY2Obfs != "" {
			obfs = fmt.Sprintf("    obfs: salamander\n    obfs-password: %s\n", st.HY2Obfs)
		}
		return fmt.Sprintf("  - name: %q\n    type: hysteria2\n    server: %s\n    port: %d\n    password: %s\n    sni: %s\n%s%s", name, host, port, u.Password, sni, skip, obfs)
	case models.ProtoTUIC:
		return fmt.Sprintf("  - name: %q\n    type: tuic\n    server: %s\n    port: %d\n    uuid: %s\n    password: %s\n    sni: %s\n    congestion-controller: bbr\n%s", name, host, port, u.UUID, u.Password, sni, skip)
	case models.ProtoShadowTLS:
		return fmt.Sprintf("  - name: %q\n    type: ss\n    server: %s\n    port: %d\n    cipher: 2022-blake3-aes-128-gcm\n    password: %s\n    plugin: shadow-tls\n    plugin-opts:\n      host: %s\n      password: %s\n      version: 3\n", name, host, port, st.SSPassword+":"+u.SSPassword, st.RealityDest(), u.Password)
	default:
		return ""
	}
}

func clashTransport(b *strings.Builder, spec models.Inbound, host string) {
	path := spec.Path
	switch spec.Transport {
	case models.TransportWS:
		fmt.Fprintf(b, "    network: ws\n    ws-opts:\n      path: %s\n      headers:\n        Host: %s\n", path, host)
	case models.TransportGRPC:
		fmt.Fprintf(b, "    network: grpc\n    grpc-opts:\n      grpc-service-name: %q\n", strings.TrimPrefix(path, "/"))
	case models.TransportHTTPUpgrade:
		fmt.Fprintf(b, "    network: ws\n    ws-opts:\n      path: %s\n      v2ray-http-upgrade: true\n      headers:\n        Host: %s\n", path, host)
	case models.TransportH2, models.TransportXHTTP:
		fmt.Fprintf(b, "    network: h2\n    h2-opts:\n      path: %s\n      host:\n        - %s\n", path, host)
	}
}

func sanitizeName(s string) string {
	s = strings.ReplaceAll(s, "|", "-")
	s = strings.TrimSpace(s)
	if s == "" {
		return "proxy"
	}
	return s
}

func SingBoxClient(u models.User, st models.Settings, domains []models.Domain, inbounds []models.Inbound) ([]byte, error) {
	links := UserLinks(u, st, domains, inbounds)
	var outbounds []map[string]any
	var tags []string
	for _, spec := range inbounds {
		if !spec.Enable {
			continue
		}
		obs := clientOutbounds(u, spec, st, domains)
		if len(obs) == 0 {
			continue
		}
		outbounds = append(outbounds, obs...)
		tags = append(tags, spec.Tag)
	}
	_ = links
	outbounds = append([]map[string]any{{
		"type": "selector", "tag": "proxy", "outbounds": tags, "default": first(tags),
	}}, outbounds...)
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": "direct"})
	cfg := map[string]any{
		"log":       map[string]any{"level": "info"},
		"outbounds": outbounds,
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func first(s []string) string {
	if len(s) == 0 {
		return "direct"
	}
	return s[0]
}

func clientOutbounds(u models.User, spec models.Inbound, st models.Settings, domains []models.Domain) []map[string]any {
	if spec.Protocol == models.ProtoShadowTLS {
		return shadowTLSClient(u, spec, st, domains)
	}
	ob := clientOutbound(u, spec, st, domains)
	if ob == nil {
		return nil
	}
	return []map[string]any{ob}
}

func shadowTLSClient(u models.User, spec models.Inbound, st models.Settings, domains []models.Domain) []map[string]any {
	host, _ := hostFor(st, domains, spec.Mode)
	port := spec.ListenPort
	if port == 0 {
		port = 8444
	}
	stTag := spec.Tag + "-out"
	return []map[string]any{
		{
			"type": "shadowtls", "tag": stTag,
			"server": host, "server_port": port, "version": 3,
			"password": u.Password,
			"tls": map[string]any{
				"enabled": true, "server_name": st.RealityDest(),
				"utls": map[string]any{"enabled": true, "fingerprint": "chrome"},
			},
		},
		{
			"type": "shadowsocks", "tag": spec.Tag, "detour": stTag,
			"method":   "2022-blake3-aes-128-gcm",
			"password": st.SSPassword + ":" + u.SSPassword,
		},
	}
}

func clientOutbound(u models.User, spec models.Inbound, st models.Settings, domains []models.Domain) map[string]any {
	host, sni := hostFor(st, domains, spec.Mode)
	port := spec.ListenPort
	if spec.Mode == models.ModeCDN {
		port = 443
	}
	if port == 0 {
		port = 443
	}
	switch spec.Protocol {
	case models.ProtoVLESS:
		ob := map[string]any{
			"type": "vless", "tag": spec.Tag, "server": host, "server_port": port,
			"uuid": u.UUID, "packet_encoding": "xudp",
		}
		if spec.Security == models.SecurityReality && spec.Transport == models.TransportTCP {
			ob["flow"] = "xtls-rprx-vision"
		}
		applyClientTLS(ob, spec, st, sni)
		applyClientTransport(ob, spec, sni)
		return ob
	case models.ProtoVMess:
		ob := map[string]any{
			"type": "vmess", "tag": spec.Tag, "server": host, "server_port": port,
			"uuid": u.UUID, "security": "auto",
		}
		applyClientTLS(ob, spec, st, sni)
		applyClientTransport(ob, spec, sni)
		return ob
	case models.ProtoTrojan:
		ob := map[string]any{
			"type": "trojan", "tag": spec.Tag, "server": host, "server_port": port,
			"password": u.Password,
		}
		applyClientTLS(ob, spec, st, sni)
		applyClientTransport(ob, spec, sni)
		return ob
	case models.ProtoHysteria2:
		ob := map[string]any{
			"type": "hysteria2", "tag": spec.Tag, "server": host, "server_port": port,
			"password": u.Password,
			"tls":      map[string]any{"enabled": true, "server_name": sni, "insecure": tlsInsecure(st, spec, sni)},
		}
		if st.HY2Obfs != "" {
			ob["obfs"] = map[string]any{"type": "salamander", "password": st.HY2Obfs}
		}
		return ob
	case models.ProtoTUIC:
		return map[string]any{
			"type": "tuic", "tag": spec.Tag, "server": host, "server_port": port,
			"uuid": u.UUID, "password": u.Password, "congestion_control": "bbr",
			"tls": map[string]any{"enabled": true, "server_name": sni, "insecure": tlsInsecure(st, spec, sni)},
		}
	default:
		return nil
	}
}

func applyClientTLS(ob map[string]any, spec models.Inbound, st models.Settings, sni string) {
	switch {
	case spec.Security == models.SecurityReality:
		ob["tls"] = map[string]any{
			"enabled": true, "server_name": st.RealityDest(),
			"utls":    map[string]any{"enabled": true, "fingerprint": "chrome"},
			"reality": map[string]any{"enabled": true, "public_key": st.RealityPublicKey, "short_id": st.RealityShortID},
		}
	case spec.Mode == models.ModeCDN || spec.Security == models.SecurityTLS:
		tls := map[string]any{
			"enabled": true, "server_name": sni,
			"utls":     map[string]any{"enabled": true, "fingerprint": "chrome"},
			"alpn":     []string{linkALPN(spec)},
			"insecure": tlsInsecure(st, spec, sni),
		}
		ob["tls"] = tls
	}
}

func applyClientTransport(ob map[string]any, spec models.Inbound, host string) {
	path := spec.Path
	if path == "" {
		path = "/" + spec.Tag
	}
	switch spec.Transport {
	case models.TransportWS:
		ob["transport"] = map[string]any{"type": "ws", "path": path, "headers": map[string]any{"Host": host}}
	case models.TransportGRPC:
		ob["transport"] = map[string]any{"type": "grpc", "service_name": strings.TrimPrefix(path, "/")}
	case models.TransportHTTPUpgrade:
		ob["transport"] = map[string]any{"type": "httpupgrade", "path": path, "headers": map[string]any{"Host": host}}
	case models.TransportH2, models.TransportXHTTP:
		ob["transport"] = map[string]any{"type": "http", "path": path, "headers": map[string]any{"Host": host}}
	}
}

func SplitHostPort(hostport string) (string, int) {
	h, p, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport, 0
	}
	port, _ := strconv.Atoi(p)
	return h, port
}
