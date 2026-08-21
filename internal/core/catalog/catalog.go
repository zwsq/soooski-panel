package catalog

import "github.com/zwsq/soooski-panel/internal/models"

type Spec struct {
	Tag             string
	Protocol        string
	Transport       string
	Security        string
	Mode            string
	ListenPort      int
	InternalPort    int
	Path            string
	NeedsRandomPath bool
	Remark          string
}

func cdn(tag, proto, transport, remark string, internal int) Spec {
	return Spec{
		Tag: tag, Protocol: proto, Transport: transport, Security: models.SecurityTLS,
		Mode: models.ModeCDN, InternalPort: internal, NeedsRandomPath: true, Remark: remark,
	}
}

func flex(tag, proto, transport, remark string, internal int) Spec {
	return Spec{
		Tag: tag, Protocol: proto, Transport: transport, Security: models.SecurityNone,
		Mode: models.ModeCDN, ListenPort: 80, InternalPort: internal, NeedsRandomPath: true, Remark: remark,
	}
}

func dirMux(tag, proto, transport, remark string, internal int) Spec {
	return Spec{
		Tag: tag, Protocol: proto, Transport: transport, Security: models.SecurityTLS,
		Mode: models.ModeDirect, ListenPort: 443, InternalPort: internal, NeedsRandomPath: true, Remark: remark,
	}
}

// Defaults is the Hiddify-style matrix: Reality / QUIC / WG / raw TLS on their
// own ports, plus WS / gRPC / HTTPUpgrade / HTTP2 (h2) / xHTTP on 443 path mux
// for both direct (own domain) and CDN (edge hostname). xHTTP is sing-box
// `http` transport — native Xray xHTTP is not in sing-box 1.11.
func Defaults() []Spec {
	p, t, v := models.ProtoVLESS, models.ProtoTrojan, models.ProtoVMess
	ws, grpc, hup, h2, xh := models.TransportWS, models.TransportGRPC, models.TransportHTTPUpgrade, models.TransportH2, models.TransportXHTTP
	return []Spec{
		{Tag: "vless-reality", Protocol: p, Transport: models.TransportTCP, Security: models.SecurityReality, Mode: models.ModeDirect, ListenPort: 443, InternalPort: 12001, Remark: "VLESS + REALITY + Vision (direct TCP 443, SNI mux)"},
		{Tag: "vless-reality-grpc", Protocol: p, Transport: grpc, Security: models.SecurityReality, Mode: models.ModeDirect, ListenPort: 10443, InternalPort: 10443, Path: "grpc", Remark: "VLESS + REALITY + gRPC (direct)"},
		{Tag: "vless-reality-h2", Protocol: p, Transport: h2, Security: models.SecurityReality, Mode: models.ModeDirect, ListenPort: 10444, InternalPort: 10444, Path: "/h2", Remark: "VLESS + REALITY + HTTP/2 (direct; Hiddify Reality h2)"},
		{Tag: "vless-reality-xhttp", Protocol: p, Transport: xh, Security: models.SecurityReality, Mode: models.ModeDirect, ListenPort: 10446, InternalPort: 10446, Path: "/xhttp", Remark: "VLESS + REALITY + xHTTP (direct; sing-box HTTP/2 stand-in)"},
		{Tag: "vless-reality-httpupgrade", Protocol: p, Transport: hup, Security: models.SecurityReality, Mode: models.ModeDirect, ListenPort: 10445, InternalPort: 10445, Path: "/httpupgrade", Remark: "VLESS + REALITY + HTTPUpgrade (direct)"},
		{Tag: "hysteria2", Protocol: models.ProtoHysteria2, Transport: models.TransportUDP, Security: models.SecurityTLS, Mode: models.ModeDirect, ListenPort: 443, Remark: "Hysteria2 QUIC (direct UDP 443)"},
		{Tag: "tuic", Protocol: models.ProtoTUIC, Transport: models.TransportUDP, Security: models.SecurityTLS, Mode: models.ModeDirect, ListenPort: 8448, Remark: "TUIC v5 (direct UDP 8448)"},
		{Tag: "shadowtls", Protocol: models.ProtoShadowTLS, Transport: models.TransportShadowTLS, Security: models.SecurityShadowTLS, Mode: models.ModeDirect, ListenPort: 8444, InternalPort: 20009, Remark: "ShadowTLS v3 + Shadowsocks 2022 (direct)"},
		{Tag: "wireguard", Protocol: models.ProtoWireGuard, Transport: models.TransportUDP, Security: models.SecurityNone, Mode: models.ModeDirect, ListenPort: 51820, Remark: "WireGuard (direct UDP 51820)"},
		{Tag: "vless-tcp", Protocol: p, Transport: models.TransportTCP, Security: models.SecurityTLS, Mode: models.ModeDirect, ListenPort: 8447, Remark: "VLESS + TCP + TLS (direct)"},
		{Tag: "trojan-tcp", Protocol: t, Transport: models.TransportTCP, Security: models.SecurityTLS, Mode: models.ModeDirect, ListenPort: 8445, Remark: "Trojan TCP + TLS (direct)"},
		{Tag: "vmess-tcp", Protocol: v, Transport: models.TransportTCP, Security: models.SecurityTLS, Mode: models.ModeDirect, ListenPort: 8446, Remark: "VMess TCP + TLS (direct)"},

		dirMux("vless-ws-direct", p, ws, "VLESS + WebSocket + TLS (direct, path mux 443)", 20020),
		dirMux("vless-grpc-direct", p, grpc, "VLESS + gRPC + TLS (direct, path mux 443)", 20021),
		dirMux("vless-h2-direct", p, h2, "VLESS + HTTP/2 (direct, path mux 443)", 20022),
		dirMux("vless-httpupgrade-direct", p, hup, "VLESS + HTTPUpgrade + TLS (direct, path mux 443)", 20023),
		dirMux("vless-xhttp-direct", p, xh, "VLESS + xHTTP (direct; sing-box HTTP/2 stand-in)", 20032),
		dirMux("vmess-ws-direct", v, ws, "VMess + WebSocket + TLS (direct, path mux 443)", 20024),
		dirMux("vmess-grpc-direct", v, grpc, "VMess + gRPC + TLS (direct, path mux 443)", 20025),
		dirMux("vmess-h2-direct", v, h2, "VMess + HTTP/2 (direct, path mux 443)", 20026),
		dirMux("vmess-httpupgrade-direct", v, hup, "VMess + HTTPUpgrade + TLS (direct, path mux 443)", 20027),
		dirMux("vmess-xhttp-direct", v, xh, "VMess + xHTTP (direct; sing-box HTTP/2 stand-in)", 20033),
		dirMux("trojan-ws-direct", t, ws, "Trojan + WebSocket + TLS (direct, path mux 443)", 20028),
		dirMux("trojan-grpc-direct", t, grpc, "Trojan + gRPC + TLS (direct, path mux 443)", 20029),
		dirMux("trojan-h2-direct", t, h2, "Trojan + HTTP/2 (direct, path mux 443)", 20030),
		dirMux("trojan-httpupgrade-direct", t, hup, "Trojan + HTTPUpgrade + TLS (direct, path mux 443)", 20031),
		dirMux("trojan-xhttp-direct", t, xh, "Trojan + xHTTP (direct; sing-box HTTP/2 stand-in)", 20034),

		cdn("vless-ws", p, ws, "VLESS + WebSocket + TLS (CDN)", 20001),
		cdn("vless-grpc", p, grpc, "VLESS + gRPC + TLS (CDN)", 20002),
		cdn("vless-httpupgrade", p, hup, "VLESS + HTTPUpgrade + TLS (CDN)", 20003),
		cdn("vless-h2", p, h2, "VLESS + HTTP/2 (CDN; Hiddify h2)", 20010),
		cdn("vless-xhttp", p, xh, "VLESS + xHTTP (CDN; sing-box HTTP/2 stand-in)", 20013),
		cdn("trojan-ws", t, ws, "Trojan + WebSocket + TLS (CDN)", 20004),
		cdn("trojan-grpc", t, grpc, "Trojan + gRPC + TLS (CDN)", 20005),
		cdn("trojan-httpupgrade", t, hup, "Trojan + HTTPUpgrade + TLS (CDN)", 20017),
		cdn("trojan-h2", t, h2, "Trojan + HTTP/2 (CDN)", 20012),
		cdn("trojan-xhttp", t, xh, "Trojan + xHTTP (CDN; sing-box HTTP/2 stand-in)", 20015),
		cdn("vmess-ws", v, ws, "VMess + WebSocket + TLS (CDN)", 20006),
		cdn("vmess-grpc", v, grpc, "VMess + gRPC + TLS (CDN)", 20007),
		cdn("vmess-httpupgrade", v, hup, "VMess + HTTPUpgrade + TLS (CDN)", 20016),
		cdn("vmess-h2", v, h2, "VMess + HTTP/2 (CDN)", 20011),
		cdn("vmess-xhttp", v, xh, "VMess + xHTTP (CDN; sing-box HTTP/2 stand-in)", 20014),

		flex("vless-ws-http", p, ws, "VLESS + WebSocket + HTTP (CDN / Cloudflare Flexible)", 20008),
		flex("vmess-ws-http", v, ws, "VMess + WebSocket + HTTP (CDN / Cloudflare Flexible)", 20018),
		flex("trojan-ws-http", t, ws, "Trojan + WebSocket + HTTP (CDN / Cloudflare Flexible)", 20019),
		flex("vless-httpupgrade-http", p, hup, "VLESS + HTTPUpgrade + HTTP (CDN / Cloudflare Flexible)", 20035),
	}
}
