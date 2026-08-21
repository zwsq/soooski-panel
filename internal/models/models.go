package models

import (
	"strings"
	"time"
)

const (
	ModeDirect     = "direct"
	ModeCDN        = "cdn"
	ModeCamouflage = "camouflage"

	ProtoVLESS       = "vless"
	ProtoVMess       = "vmess"
	ProtoTrojan      = "trojan"
	ProtoHysteria2   = "hysteria2"
	ProtoTUIC        = "tuic"
	ProtoShadowsocks = "shadowsocks"
	ProtoShadowTLS   = "shadowtls"
	ProtoWireGuard   = "wireguard"

	TransportTCP         = "tcp"
	TransportWS          = "ws"
	TransportGRPC        = "grpc"
	TransportHTTPUpgrade = "httpupgrade"
	TransportH2          = "http"  // sing-box HTTP/2 (Hiddify "h2")
	TransportXHTTP       = "xhttp" // stored name; compiled as sing-box "http"
	TransportUDP         = "udp"
	TransportShadowTLS   = "shadowtls"

	SecurityReality   = "reality"
	SecurityTLS       = "tls"
	SecurityNone      = "none"
	SecurityShadowTLS = "shadowtls"
)

type Admin struct {
	ID           int64
	Username     string
	PasswordHash string
}

type User struct {
	ID             int64      `json:"id"`
	Username       string     `json:"username"`
	UUID           string     `json:"uuid"`
	Password       string     `json:"-"`
	SSPassword     string     `json:"-"`
	SubToken       string     `json:"sub_token"`
	Enable         bool       `json:"enable"`
	ExpireAt       *time.Time `json:"expire_at,omitempty"`
	TrafficLimit   int64      `json:"traffic_limit"`
	TrafficUp      int64      `json:"traffic_up"`
	TrafficDown    int64      `json:"traffic_down"`
	WGPrivateKey   string     `json:"-"`
	WGPublicKey    string     `json:"wg_public_key"`
	WGIP           string     `json:"wg_ip"`
	Note           string     `json:"note"`
	TelegramSecret string     `json:"telegram_secret,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

func (u User) Expired() bool {
	return u.ExpireAt != nil && time.Now().After(*u.ExpireAt)
}

func (u User) OverQuota() bool {
	if u.TrafficLimit <= 0 {
		return false
	}
	return u.TrafficUp+u.TrafficDown >= u.TrafficLimit
}

func (u User) Active() bool {
	return u.Enable && !u.Expired() && !u.OverQuota()
}

type Domain struct {
	ID       int64  `json:"id"`
	Domain   string `json:"domain"`
	Mode     string `json:"mode"`
	Enable   bool   `json:"enable"`
	Provider string `json:"provider"`
}

type Inbound struct {
	ID           int64  `json:"id"`
	Tag          string `json:"tag"`
	Protocol     string `json:"protocol"`
	Transport    string `json:"transport"`
	Security     string `json:"security"`
	Mode         string `json:"mode"`
	ListenPort   int    `json:"listen_port"`
	InternalPort int    `json:"internal_port"`
	Path         string `json:"path"`
	Enable       bool   `json:"enable"`
	Remark       string `json:"remark"`
}

// PathMuxed is true when the inbound is reached through the Go 80/443 path reverse proxy.
func (in Inbound) PathMuxed() bool {
	if !in.Enable || in.InternalPort == 0 || in.Path == "" {
		return false
	}
	if in.Security == SecurityReality {
		return false
	}
	switch in.Transport {
	case TransportWS, TransportGRPC, TransportHTTPUpgrade, TransportH2, TransportXHTTP:
		return true
	default:
		return false
	}
}

// H2C is true when the reverse proxy must speak HTTP/2 cleartext to sing-box
// (gRPC, HTTP/2, and the xHTTP stand-in).
func (in Inbound) H2C() bool {
	return in.Transport == TransportGRPC || in.Transport == TransportH2 || in.Transport == TransportXHTTP
}

// SingBoxTransport is the sing-box JSON transport type.
func SingBoxTransport(t string) string {
	if t == TransportXHTTP {
		return "http"
	}
	return t
}

func HTTP2Transport(t string) bool {
	return t == TransportH2 || t == TransportXHTTP
}

type Settings struct {
	PublicHost         string `json:"public_host"`
	RealityPrivateKey  string `json:"-"`
	RealityPublicKey   string `json:"reality_public_key"`
	RealityShortID     string `json:"reality_short_id"`
	RealityServerName  string `json:"reality_server_name"`
	SSPassword         string `json:"-"`
	WGPrivateKey       string `json:"-"`
	WGPublicKey        string `json:"wg_public_key"`
	WGSubnet           string `json:"wg_subnet"`
	HY2Obfs            string `json:"hy2_obfs"`
	ClashSecret        string `json:"-"`
	CamouflageURL      string `json:"camouflage_url"`
	TLSCertPath        string `json:"tls_cert_path"`
	TLSKeyPath         string `json:"tls_key_path"`
	ACMEEmail          string `json:"acme_email"`
	AdminPath          string `json:"admin_path"`
	ClientPath         string `json:"client_path"`
	TelegramEnabled    bool   `json:"telegram_enabled"`
	TelegramFakeDomain string `json:"telegram_fake_domain"`
	TelegramSecret     string `json:"telegram_secret"`
}

func (s Settings) AdminPrefix() string {
	p := strings.Trim(s.AdminPath, "/")
	if p == "" {
		return "/panel"
	}
	return "/" + p
}

func (s Settings) ClientPrefix() string {
	p := strings.Trim(s.ClientPath, "/")
	if p == "" {
		return "/sub"
	}
	return "/" + p
}

type CertStatus struct {
	Domain    string `json:"domain"`
	State     string `json:"state"` // issued, renewing, pending, failed, ineligible, self_signed, missing
	Issuer    string `json:"issuer,omitempty"`
	NotBefore string `json:"not_before,omitempty"`
	NotAfter  string `json:"not_after,omitempty"`
	DaysLeft  int    `json:"days_left"`
	Error     string `json:"error,omitempty"`
	AutoRenew bool   `json:"auto_renew"`
}

type Dashboard struct {
	CoreRunning  bool         `json:"core_running"`
	CoreError    string       `json:"core_error,omitempty"`
	UsersTotal   int          `json:"users_total"`
	UsersActive  int          `json:"users_active"`
	TrafficUp    int64        `json:"traffic_up"`
	TrafficDown  int64        `json:"traffic_down"`
	TrafficError string       `json:"traffic_error,omitempty"`
	PublicHost   string       `json:"public_host"`
	InboundsOn   int          `json:"inbounds_on"`
	Domains      int          `json:"domains"`
	AdminURL     string       `json:"admin_url"`
	ClientPath   string       `json:"client_path"`
	DataDir      string       `json:"data_dir"`
	Certs        []CertStatus `json:"certs"`
	ACMEEmail    string       `json:"acme_email,omitempty"`
}
