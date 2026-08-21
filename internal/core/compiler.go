package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/zwsq/soooski-panel/internal/models"
)

type CompileInput struct {
	Settings models.Settings
	Users    []models.User
	Inbounds []models.Inbound
}

func activeUsers(users []models.User) []models.User {
	var out []models.User
	for _, u := range users {
		if u.Active() {
			out = append(out, u)
		}
	}
	return out
}

func Compile(in CompileInput) ([]byte, error) {
	users := activeUsers(in.Users)
	st := in.Settings
	inbounds := make([]map[string]any, 0, len(in.Inbounds)+2)
	var endpoints []map[string]any
	for _, spec := range in.Inbounds {
		if !spec.Enable {
			continue
		}
		if spec.Protocol == models.ProtoWireGuard {
			if ep := wireguardEndpoint(spec, st, users); ep != nil {
				endpoints = append(endpoints, ep)
			}
			continue
		}
		ib, err := compileInbound(spec, st, users)
		if err != nil {
			return nil, err
		}
		if ib != nil {
			inbounds = append(inbounds, ib...)
		}
	}
	cfg := map[string]any{
		"log": map[string]any{"level": "info", "timestamp": true},
		// ipv4_only: REALITY dest (e.g. www.microsoft.com) is often Akamai
		// dual-stack. Hosts without working IPv6 fail with
		// "dial tcp [2a02:...]:443: network is unreachable".
		"dns": map[string]any{
			"servers":  []map[string]any{{"tag": "google", "address": "8.8.8.8", "detour": "direct"}},
			"strategy": "ipv4_only",
			"final":    "google",
		},
		"inbounds":  inbounds,
		"outbounds": userOutbounds(users),
		"route": map[string]any{
			"rules":                 routeRules(users),
			"final":                 "direct",
			"auto_detect_interface": true,
		},
		"experimental": map[string]any{
			"clash_api": map[string]any{
				"external_controller": "127.0.0.1:9090",
				"secret":              st.ClashSecret,
			},
		},
	}
	if len(endpoints) > 0 {
		cfg["endpoints"] = endpoints
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func userOutboundTag(u models.User) string {
	if u.ID > 0 {
		return fmt.Sprintf("user-%d", u.ID)
	}
	return "user-" + u.Username
}

func userOutbounds(users []models.User) []map[string]any {
	out := []map[string]any{
		{"type": "direct", "tag": "direct", "domain_strategy": "ipv4_only"},
	}
	for _, u := range users {
		out = append(out, map[string]any{
			"type":            "direct",
			"tag":             userOutboundTag(u),
			"domain_strategy": "ipv4_only",
		})
	}
	return out
}

func routeRules(users []models.User) []map[string]any {
	rules := []map[string]any{
		{"action": "sniff"},
		{"protocol": "dns", "action": "hijack-dns"},
	}
	for _, u := range users {
		rules = append(rules, map[string]any{
			"auth_user": []string{u.Username},
			"outbound":  userOutboundTag(u),
		})
	}
	return rules
}

func compileInbound(spec models.Inbound, st models.Settings, users []models.User) ([]map[string]any, error) {
	switch spec.Protocol {
	case models.ProtoVLESS:
		return []map[string]any{vlessInbound(spec, st, users)}, nil
	case models.ProtoVMess:
		return []map[string]any{vmessInbound(spec, st, users)}, nil
	case models.ProtoTrojan:
		return []map[string]any{trojanInbound(spec, st, users)}, nil
	case models.ProtoHysteria2:
		return []map[string]any{hy2Inbound(spec, st, users)}, nil
	case models.ProtoTUIC:
		return []map[string]any{tuicInbound(spec, st, users)}, nil
	case models.ProtoShadowTLS:
		return shadowTLS(spec, st, users), nil
	case models.ProtoWireGuard:
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown protocol %s", spec.Protocol)
	}
}

func listenAddr(spec models.Inbound) (string, int) {
	if spec.InternalPort > 0 && spec.InternalPort != spec.ListenPort {
		return "127.0.0.1", spec.InternalPort
	}
	if spec.Mode == models.ModeCDN && spec.InternalPort > 0 {
		return "127.0.0.1", spec.InternalPort
	}
	port := spec.ListenPort
	if port == 0 {
		port = spec.InternalPort
	}
	return "::", port
}

func vlessUsers(users []models.User, flow string) []map[string]any {
	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		m := map[string]any{"name": u.Username, "uuid": u.UUID}
		if flow != "" {
			m["flow"] = flow
		}
		out = append(out, m)
	}
	return out
}

func vlessInbound(spec models.Inbound, st models.Settings, users []models.User) map[string]any {
	host, port := listenAddr(spec)
	flow := ""
	if spec.Security == models.SecurityReality && spec.Transport == models.TransportTCP {
		flow = "xtls-rprx-vision"
	}
	ib := map[string]any{
		"type":        "vless",
		"tag":         spec.Tag,
		"listen":      host,
		"listen_port": port,
		"users":       vlessUsers(users, flow),
		"sniff":       true,
	}
	applyTLS(ib, spec, st)
	applyTransport(ib, spec)
	return ib
}

func vmessInbound(spec models.Inbound, st models.Settings, users []models.User) map[string]any {
	host, port := listenAddr(spec)
	vu := make([]map[string]any, 0, len(users))
	for _, u := range users {
		vu = append(vu, map[string]any{"name": u.Username, "uuid": u.UUID, "alterId": 0})
	}
	ib := map[string]any{
		"type":        "vmess",
		"tag":         spec.Tag,
		"listen":      host,
		"listen_port": port,
		"users":       vu,
		"sniff":       true,
	}
	applyTLS(ib, spec, st)
	applyTransport(ib, spec)
	return ib
}

func trojanInbound(spec models.Inbound, st models.Settings, users []models.User) map[string]any {
	host, port := listenAddr(spec)
	tu := make([]map[string]any, 0, len(users))
	for _, u := range users {
		tu = append(tu, map[string]any{"name": u.Username, "password": u.Password})
	}
	ib := map[string]any{
		"type":        "trojan",
		"tag":         spec.Tag,
		"listen":      host,
		"listen_port": port,
		"users":       tu,
		"sniff":       true,
	}
	applyTLS(ib, spec, st)
	applyTransport(ib, spec)
	return ib
}

func hy2Inbound(spec models.Inbound, st models.Settings, users []models.User) map[string]any {
	hu := make([]map[string]any, 0, len(users))
	for _, u := range users {
		hu = append(hu, map[string]any{"name": u.Username, "password": u.Password})
	}
	tls := tlsCert(st)
	tls["alpn"] = []string{"h3"}
	ib := map[string]any{
		"type":        "hysteria2",
		"tag":         spec.Tag,
		"listen":      "::",
		"listen_port": spec.ListenPort,
		"users":       hu,
		"tls":         tls,
		"masquerade":  "https://www.microsoft.com",
	}
	if st.HY2Obfs != "" {
		ib["obfs"] = map[string]any{"type": "salamander", "password": st.HY2Obfs}
	}
	return ib
}

func tuicInbound(spec models.Inbound, st models.Settings, users []models.User) map[string]any {
	tu := make([]map[string]any, 0, len(users))
	for _, u := range users {
		tu = append(tu, map[string]any{"name": u.Username, "uuid": u.UUID, "password": u.Password})
	}
	return map[string]any{
		"type":               "tuic",
		"tag":                spec.Tag,
		"listen":             "::",
		"listen_port":        spec.ListenPort,
		"users":              tu,
		"congestion_control": "bbr",
		"tls":                tlsCert(st),
	}
}

func shadowTLS(spec models.Inbound, st models.Settings, users []models.User) []map[string]any {
	ssUsers := make([]map[string]any, 0, len(users))
	stUsers := make([]map[string]any, 0, len(users))
	for _, u := range users {
		ssUsers = append(ssUsers, map[string]any{"name": u.Username, "password": u.SSPassword})
		stUsers = append(stUsers, map[string]any{"name": u.Username, "password": u.Password})
	}
	ssPort := spec.InternalPort
	if ssPort == 0 {
		ssPort = 20009
	}
	ss := map[string]any{
		"type":        "shadowsocks",
		"tag":         "ss-inner",
		"listen":      "127.0.0.1",
		"listen_port": ssPort,
		"method":      "2022-blake3-aes-128-gcm",
		"password":    st.SSPassword,
		"users":       ssUsers,
	}
	stls := map[string]any{
		"type":        "shadowtls",
		"tag":         spec.Tag,
		"listen":      "::",
		"listen_port": spec.ListenPort,
		"version":     3,
		"users":       stUsers,
		"handshake":   handshakeDest(st),
		"detour":      "ss-inner",
	}
	return []map[string]any{ss, stls}
}

func wireguardEndpoint(spec models.Inbound, st models.Settings, users []models.User) map[string]any {
	if st.WGPrivateKey == "" {
		return nil
	}
	peers := make([]map[string]any, 0, len(users))
	for _, u := range users {
		if u.WGPublicKey == "" {
			continue
		}
		allowed := u.WGIP
		if allowed != "" && !strings.Contains(allowed, "/") {
			allowed += "/32"
		}
		peers = append(peers, map[string]any{
			"public_key":  u.WGPublicKey,
			"allowed_ips": []string{allowed},
		})
	}
	addr := wgServerAddress(st.WGSubnet)
	return map[string]any{
		"type":        "wireguard",
		"tag":         spec.Tag,
		"system":      false,
		"mtu":         1408,
		"address":     []string{addr},
		"private_key": st.WGPrivateKey,
		"listen_port": spec.ListenPort,
		"peers":       peers,
	}
}

func wgServerAddress(subnet string) string {
	if subnet == "" {
		return "10.66.66.1/24"
	}
	parts := strings.Split(subnet, "/")
	ip := parts[0]
	mask := "24"
	if len(parts) > 1 {
		mask = parts[1]
	}
	if strings.HasSuffix(ip, ".0") {
		return strings.TrimSuffix(ip, ".0") + ".1/" + mask
	}
	return ip + "/" + mask
}

func tlsCert(st models.Settings) map[string]any {
	return map[string]any{
		"enabled":          true,
		"certificate_path": st.TLSCertPath,
		"key_path":         st.TLSKeyPath,
	}
}

func handshakeDest(st models.Settings) map[string]any {
	sni := strings.TrimSpace(st.RealityServerName)
	if sni == "" {
		sni = "www.microsoft.com"
	}
	return map[string]any{
		"server":          sni,
		"server_port":     443,
		"domain_strategy": "ipv4_only",
		"connect_timeout": "5s",
	}
}

func applyTLS(ib map[string]any, spec models.Inbound, st models.Settings) {
	switch spec.Security {
	case models.SecurityReality:
		sni := strings.TrimSpace(st.RealityServerName)
		if sni == "" {
			sni = "www.microsoft.com"
		}
		ib["tls"] = map[string]any{
			"enabled":     true,
			"server_name": sni,
			"reality": map[string]any{
				"enabled":     true,
				"private_key": st.RealityPrivateKey,
				"short_id":    []string{st.RealityShortID},
				"handshake":   handshakeDest(st),
			},
		}
	case models.SecurityTLS:
		// Path-muxed inbounds sit behind Go TLS on 443/80. Do not wrap
		// sing-box in a second TLS layer (WS/gRPC/H2 speak cleartext/h2c).
		if spec.PathMuxed() {
			return
		}
		ib["tls"] = tlsCert(st)
	}
}

func applyTransport(ib map[string]any, spec models.Inbound) {
	path := spec.Path
	if path == "" {
		path = "/" + spec.Tag
	}
	if !strings.HasPrefix(path, "/") && spec.Transport != models.TransportGRPC {
		path = "/" + path
	}
	svc := strings.TrimPrefix(path, "/")
	switch spec.Transport {
	case models.TransportWS:
		ib["transport"] = map[string]any{
			"type":                   "ws",
			"path":                   path,
			"max_early_data":         2048,
			"early_data_header_name": "Sec-WebSocket-Protocol",
		}
	case models.TransportGRPC:
		ib["transport"] = map[string]any{"type": "grpc", "service_name": svc}
	case models.TransportHTTPUpgrade:
		ib["transport"] = map[string]any{"type": "httpupgrade", "path": path}
	case models.TransportH2, models.TransportXHTTP:
		ib["transport"] = map[string]any{"type": "http", "path": path}
	}
}
