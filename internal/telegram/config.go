package telegram

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/zwsq/soooski-panel/internal/crypto"
	"github.com/zwsq/soooski-panel/internal/models"
)

const (
	DefaultFakeDomain = "www.cloudflare.com"
	ListenAddr        = "127.0.0.1:1001"
	ListenPort        = 1001
	StatsAddr         = "127.0.0.1:1002"
	StatsURL          = "http://127.0.0.1:1002/stats"
	ReloadURL         = "http://127.0.0.1:1002/reload"
)

func SecretName(id int64) string {
	return "u" + strconv.FormatInt(id, 10)
}

func ParseSecretName(name string) (int64, bool) {
	name = strings.TrimSpace(name)
	if !strings.HasPrefix(name, "u") {
		return 0, false
	}
	id, err := strconv.ParseInt(name[1:], 10, 64)
	return id, err == nil && id > 0
}

func NormalizeFakeDomain(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimPrefix(v, "http://")
	if i := strings.IndexAny(v, "/:"); i >= 0 {
		v = v[:i]
	}
	v = strings.TrimSuffix(strings.TrimSpace(v), ".")
	if v == "" {
		return DefaultFakeDomain
	}
	return v
}

func ValidateFakeDomain(domain string, ours []string) error {
	domain = NormalizeFakeDomain(domain)
	if net.ParseIP(domain) != nil {
		return fmt.Errorf("telegram fake domain must be a public website hostname, not an IP")
	}
	if !strings.Contains(domain, ".") {
		return fmt.Errorf("telegram fake domain must be a hostname")
	}
	for _, h := range ours {
		h = strings.ToLower(strings.TrimSpace(h))
		h = strings.TrimSuffix(h, ".")
		if h != "" && h == domain {
			return fmt.Errorf("do not use %s as the Telegram fake domain (it would steal that SNI from the panel or REALITY)", domain)
		}
	}
	return nil
}

func RenderTOML(st models.Settings, users []models.User) string {
	var b strings.Builder
	b.WriteString("debug = false\n")
	fmt.Fprintf(&b, "bind-to = %q\n", ListenAddr)
	fmt.Fprintf(&b, "api-bind-to = %q\n", StatsAddr)
	b.WriteString("prefer-ip = \"only-ipv4\"\n")
	b.WriteString("auto-update = false\n")
	b.WriteString("tolerate-time-skewness = \"5s\"\n")
	b.WriteString("allow-fallback-on-unknown-dc = true\n")
	if ip := net.ParseIP(strings.TrimSpace(st.PublicHost)); ip != nil && ip.To4() != nil {
		fmt.Fprintf(&b, "public-ipv4 = %q\n", ip.String())
	}
	b.WriteString("\n[network]\n")
	b.WriteString("dns = \"udp://1.1.1.1\"\n")
	b.WriteString("\n[defense.blocklist]\n")
	b.WriteString("enabled = false\n")
	b.WriteString("\n[stats.prometheus]\n")
	b.WriteString("enabled = false\n")
	b.WriteString("\n[secrets]\n")
	domain := NormalizeFakeDomain(st.TelegramFakeDomain)
	for _, u := range users {
		if !u.Active() || u.TelegramSecret == "" {
			continue
		}
		if !crypto.TelegramSecretMatchesDomain(u.TelegramSecret, domain) {
			continue
		}
		fmt.Fprintf(&b, "%s = %q\n", SecretName(u.ID), u.TelegramSecret)
	}
	return b.String()
}
