package api

import (
	"bytes"
	"encoding/base64"
	"html/template"
	"net/http"
	"strings"

	"github.com/skip2/go-qrcode"
	"github.com/zwsq/soooski-panel/internal/core"
	"github.com/zwsq/soooski-panel/internal/models"
	"github.com/zwsq/soooski-panel/internal/web"
)

func isBrowser(ua string) bool {
	ua = strings.ToLower(ua)
	if ua == "" {
		return false
	}
	for _, n := range []string{"clash", "stash", "meta", "v2ray", "v2rayng", "sing-box", "singbox", "hiddify", "okhttp", "sfa", "sfavn", "shadowrocket", "quantumult", "surge", "nekobox", "streisand"} {
		if strings.Contains(ua, n) {
			return false
		}
	}
	return strings.Contains(ua, "mozilla") || strings.Contains(ua, "chrome") || strings.Contains(ua, "safari") || strings.Contains(ua, "firefox")
}

func formatFloat1(f float64) string {
	n := int(f*10 + 0.5)
	whole := n / 10
	frac := n % 10
	if frac == 0 {
		return itoa(whole)
	}
	return itoa(whole) + "." + itoa(frac)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func formatBytes(n int64) string {
	if n <= 0 {
		return "0 B"
	}
	u := []string{"B", "KB", "MB", "GB", "TB"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(u)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return itoa(int(f)) + " B"
	}
	return formatFloat1(f) + " " + u[i]
}

func qrPNG(payload string) string {
	if payload == "" {
		return ""
	}
	png, err := qrcode.Encode(payload, qrcode.Medium, 200)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(png)
}

type portalData struct {
	Title            string
	Username         string
	Expired          bool
	Used             string
	Limit            string
	LimitBytes       int64
	Pct              int
	Expire           string
	QR               string
	SubURL           string
	ClashPath        string
	SingPath         string
	V2Path           string
	Links            []core.Link
	TelegramEnabled  bool
	TelegramServer   string
	TelegramDomain   string
	TelegramTG       string
	TelegramHTTPS    string
	TelegramQR       string
}

func (s *Server) writeClientPortal(w http.ResponseWriter, r *http.Request, u models.User, st models.Settings, domains []models.Domain, inbounds []models.Inbound) {
	used := u.TrafficUp + u.TrafficDown
	pct := 0
	if u.TrafficLimit > 0 {
		pct = int(float64(used) / float64(u.TrafficLimit) * 100)
		if pct > 100 {
			pct = 100
		}
	}
	exp := ""
	if u.ExpireAt != nil {
		exp = u.ExpireAt.UTC().Format("2006-01-02")
	}
	prefix := st.ClientPrefix()
	subPath := prefix + "/" + u.SubToken
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if host == "" {
		host = st.PublicHost
	}
	subURL := scheme + "://" + host + subPath
	limit := ""
	if u.TrafficLimit > 0 {
		limit = formatBytes(u.TrafficLimit)
	}
	all := core.UserLinks(u, st, domains, inbounds)
	var vpn []core.Link
	var tgURI, tgHTTPS string
	for _, l := range all {
		if l.Protocol == "mtproto" {
			if strings.HasPrefix(l.URI, "tg://") {
				tgURI = l.URI
			} else {
				tgHTTPS = l.URI
			}
			continue
		}
		vpn = append(vpn, l)
	}
	tgHost := strings.TrimSpace(st.PublicHost)
	if tgHost == "" {
		tgHost = host
	}
	data := portalData{
		Title: "soooski — " + u.Username, Username: u.Username, Expired: u.Expired(),
		Used: formatBytes(used), Limit: limit, LimitBytes: u.TrafficLimit, Pct: pct, Expire: exp,
		QR: qrPNG(subURL), SubURL: subURL, ClashPath: subPath + "/clash", SingPath: subPath + "/sing-box",
		V2Path: subPath + "/v2ray", Links: vpn,
		TelegramEnabled: st.TelegramEnabled && tgURI != "",
		TelegramServer:  tgHost,
		TelegramDomain:  st.TelegramFakeDomain,
		TelegramTG:      tgURI,
		TelegramHTTPS:   tgHTTPS,
		TelegramQR:      qrPNG(tgURI),
	}
	tmpl, err := template.ParseFS(web.FS, "client.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Del("content-disposition")
	_, _ = w.Write(buf.Bytes())
}
