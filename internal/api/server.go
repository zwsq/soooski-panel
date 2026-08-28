package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zwsq/soooski-panel/internal/core"
	"github.com/zwsq/soooski-panel/internal/crypto"
	"github.com/zwsq/soooski-panel/internal/models"
	"github.com/zwsq/soooski-panel/internal/store"
	"github.com/zwsq/soooski-panel/internal/supervisor"
	"github.com/zwsq/soooski-panel/internal/telegram"
	"github.com/zwsq/soooski-panel/internal/web"
)

type Server struct {
	Store      *store.Store
	Supervisor *supervisor.Supervisor
	Apply      func() error
	DataDir    string
	LogLevel   string
	TLSEmail   func(string)
	Certs      func() []models.CertStatus
	IssueCerts func() error
	TrafficErr func() string

	once sync.Once
	mux  *http.ServeMux
}

func (s *Server) Handler() http.Handler {
	s.once.Do(s.routes)
	return s.mux
}

func (s *Server) routes() {
	s.mux = http.NewServeMux()
	s.mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"ok": true, "core": s.Supervisor.Running()})
	})
	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.HandleFunc("POST /api/logout", s.logout)
	s.mux.HandleFunc("GET /api/me", s.auth(s.me))
	s.mux.HandleFunc("GET /api/dashboard", s.auth(s.dashboard))
	s.mux.HandleFunc("GET /api/settings", s.auth(s.getSettings))
	s.mux.HandleFunc("PUT /api/settings", s.auth(s.putSettings))
	s.mux.HandleFunc("PUT /api/admin", s.auth(s.putAdmin))
	s.mux.HandleFunc("GET /api/users", s.auth(s.listUsers))
	s.mux.HandleFunc("POST /api/users", s.auth(s.createUser))
	s.mux.HandleFunc("PUT /api/users/{id}", s.auth(s.updateUser))
	s.mux.HandleFunc("DELETE /api/users/{id}", s.auth(s.deleteUser))
	s.mux.HandleFunc("POST /api/users/{id}/reset-traffic", s.auth(s.resetTraffic))
	s.mux.HandleFunc("GET /api/users/{id}/links", s.auth(s.userLinks))
	s.mux.HandleFunc("GET /api/domains", s.auth(s.listDomains))
	s.mux.HandleFunc("POST /api/domains", s.auth(s.createDomain))
	s.mux.HandleFunc("PUT /api/domains/{id}", s.auth(s.updateDomain))
	s.mux.HandleFunc("DELETE /api/domains/{id}", s.auth(s.deleteDomain))
	s.mux.HandleFunc("GET /api/inbounds", s.auth(s.listInbounds))
	s.mux.HandleFunc("PUT /api/inbounds/{id}", s.auth(s.updateInbound))
	s.mux.HandleFunc("GET /api/config", s.auth(s.previewConfig))
	s.mux.HandleFunc("POST /api/apply", s.auth(s.apply))
	s.mux.HandleFunc("GET /sub/{token}", s.subscription)
	s.mux.HandleFunc("GET /sub/{token}/clash", s.subClash)
	s.mux.HandleFunc("GET /sub/{token}/sing-box", s.subSingBox)
	s.mux.HandleFunc("GET /sub/{token}/v2ray", s.subV2Ray)
	s.mux.HandleFunc("GET /sub/{token}/info", s.subInfo)
	s.mux.HandleFunc("POST /api/certs/issue", s.auth(s.issueCerts))
	s.mux.HandleFunc("GET /generate_204", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	panel, err := fs.Sub(web.FS, "dist")
	if err == nil {
		fileServer := http.FileServer(http.FS(panel))
		s.mux.Handle("GET /", fileServer)
	}
}

func (s *Server) token(r *http.Request) string {
	if c, err := r.Cookie("soooski_session"); err == nil {
		return c.Value
	}
	h := r.Header.Get("Authorization")
	return strings.TrimPrefix(h, "Bearer ")
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := s.Store.SessionAdmin(s.token(r)); err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"bad json"}`, 400)
		return
	}
	admin, err := s.Store.AdminByUsername(body.Username)
	if err != nil || !crypto.CheckPassword(admin.PasswordHash, body.Password) {
		http.Error(w, `{"error":"invalid credentials"}`, 401)
		return
	}
	token, err := s.Store.CreateSession(admin.ID, 7*24*time.Hour)
	if err != nil {
		http.Error(w, `{"error":"session"}`, 500)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "soooski_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 3600,
	})
	writeJSON(w, 200, map[string]any{"token": token, "username": admin.Username})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.Store.DeleteSession(s.token(r))
	http.SetCookie(w, &http.Cookie{Name: "soooski_session", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	a, _ := s.Store.SessionAdmin(s.token(r))
	writeJSON(w, 200, map[string]any{"username": a.Username})
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	st, users, domains, inbounds, err := s.Store.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	d := models.Dashboard{
		CoreRunning: s.Supervisor.Running(),
		CoreError:   s.Supervisor.LastError(),
		UsersTotal:  len(users),
		PublicHost:  st.PublicHost,
		Domains:     len(domains),
		AdminURL:    st.AdminPrefix() + "/",
		ClientPath:  st.ClientPrefix(),
		DataDir:     s.DataDir,
	}
	for _, u := range users {
		if u.Active() {
			d.UsersActive++
		}
		d.TrafficUp += u.TrafficUp
		d.TrafficDown += u.TrafficDown
	}
	if s.TrafficErr != nil {
		d.TrafficError = s.TrafficErr()
	}
	if s.Certs != nil {
		d.Certs = s.Certs()
	}
	d.ACMEEmail = st.ACMEEmail
	for _, in := range inbounds {
		if in.Enable {
			d.InboundsOn++
		}
	}
	writeJSON(w, 200, d)
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.Settings()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, st)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"bad json"}`, 400)
		return
	}
	allowed := map[string]string{
		"public_host":         "public_host",
		"reality_server":      "reality_server",
		"hy2_obfs":            "hy2_obfs",
		"camouflage_url":      "camouflage_url",
		"reality_server_name": "reality_server",
		"acme_email":          "acme_email",
	}
	if v, ok := body["admin_path"]; ok {
		p, err := normalizeSecretPath(v)
		if err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
		if err := s.Store.SetSetting("admin_path", p); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		s.writeAdminURL()
	}
	for k, v := range body {
		if key, ok := allowed[k]; ok {
			if err := s.Store.SetSetting(key, v); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
	}
	if s.TLSEmail != nil {
		if v, ok := body["acme_email"]; ok {
			s.TLSEmail(v)
		}
	}
	if telegramKeys(body) {
		if err := s.applyTelegramSettings(body); err != nil {
			writeJSON(w, 400, map[string]any{"error": err.Error()})
			return
		}
	}
	_ = s.Apply()
	s.kickCerts()
	s.getSettings(w, r)
}

func telegramKeys(body map[string]string) bool {
	_, a := body["telegram_enabled"]
	_, b := body["telegram_fake_domain"]
	_, c := body["telegram_regenerate"]
	return a || b || c
}

func (s *Server) applyTelegramSettings(body map[string]string) error {
	st, err := s.Store.Settings()
	if err != nil {
		return err
	}
	enabled := st.TelegramEnabled
	if v, ok := body["telegram_enabled"]; ok {
		enabled = v == "1" || strings.EqualFold(v, "true")
	}
	domain := st.TelegramFakeDomain
	if v, ok := body["telegram_fake_domain"]; ok {
		domain = telegram.NormalizeFakeDomain(v)
	}
	if domain == "" {
		domain = telegram.DefaultFakeDomain
	}
	ours := []string{st.PublicHost, st.RealityServerName}
	domains, err := s.Store.Domains()
	if err != nil {
		return err
	}
	for _, d := range domains {
		if d.Enable {
			ours = append(ours, d.Domain)
		}
	}
	if err := telegram.ValidateFakeDomain(domain, ours); err != nil {
		return err
	}
	regen := body["telegram_regenerate"] == "1" || strings.EqualFold(body["telegram_regenerate"], "true")
	domainChanged := !strings.EqualFold(domain, st.TelegramFakeDomain)
	if err := s.Store.SetSetting("telegram_enabled", boolSetting(enabled)); err != nil {
		return err
	}
	if err := s.Store.SetSetting("telegram_fake_domain", domain); err != nil {
		return err
	}
	if regen || domainChanged {
		return s.Store.RotateTelegramSecrets(domain)
	}
	return s.Store.EnsureTelegramSecrets()
}

func boolSetting(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func (s *Server) writeAdminURL() {
	if s.DataDir == "" {
		return
	}
	st, err := s.Store.Settings()
	if err != nil {
		return
	}
	host := strings.TrimSpace(st.PublicHost)
	if host == "" {
		host = "localhost"
	}
	_ = os.WriteFile(filepath.Join(s.DataDir, "admin-url.txt"), []byte("https://"+host+st.AdminPrefix()+"/\n"), 0600)
}

func normalizeSecretPath(v string) (string, error) {
	v = strings.TrimSpace(v)
	v = strings.Trim(v, "/")
	for strings.Contains(v, "//") {
		v = strings.ReplaceAll(v, "//", "/")
	}
	if v == "" || strings.Contains(v, "..") {
		return "", fmt.Errorf("admin path is required")
	}
	parts := strings.Split(v, "/")
	if len(v) < 4 {
		return "", fmt.Errorf("admin path must be at least 4 characters")
	}
	reserved := map[string]bool{
		"api": true, "sub": true, "healthz": true, "generate_204": true,
		"favicon.svg": true, "favicon.ico": true, "logo.svg": true, "apple-touch-icon.svg": true,
	}
	for _, part := range parts {
		if part == "" || reserved[strings.ToLower(part)] {
			return "", fmt.Errorf("admin path uses a reserved name")
		}
		for _, c := range part {
			ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-'
			if !ok {
				return "", fmt.Errorf("admin path may contain letters, digits, _ - and /")
			}
		}
	}
	return v, nil
}

func parseExpireDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return nil, err
	}
	t = t.Add(24*time.Hour - time.Second)
	return &t, nil
}

func (s *Server) putAdmin(w http.ResponseWriter, r *http.Request) {
	admin, err := s.Store.SessionAdmin(s.token(r))
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var body struct {
		CurrentPassword string `json:"current_password"`
		Username        string `json:"username"`
		Password        string `json:"password"`
		PasswordConfirm string `json:"password_confirm"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"bad json"}`, 400)
		return
	}
	if body.CurrentPassword == "" || !crypto.CheckPassword(admin.PasswordHash, body.CurrentPassword) {
		http.Error(w, `{"error":"current password is wrong"}`, 401)
		return
	}
	username := strings.TrimSpace(body.Username)
	password := body.Password
	if username == "" && password == "" {
		http.Error(w, `{"error":"set a new username or password"}`, 400)
		return
	}
	if username != "" {
		if len(username) < 3 || len(username) > 64 {
			http.Error(w, `{"error":"username must be 3-64 characters"}`, 400)
			return
		}
		for _, c := range username {
			ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-'
			if !ok {
				http.Error(w, `{"error":"username may contain letters, digits, . _ -"}`, 400)
				return
			}
		}
	}
	hash := ""
	if password != "" {
		if len(password) < 8 {
			http.Error(w, `{"error":"new password must be at least 8 characters"}`, 400)
			return
		}
		if body.PasswordConfirm != password {
			http.Error(w, `{"error":"passwords do not match"}`, 400)
			return
		}
		hash, err = crypto.HashPassword(password)
		if err != nil {
			http.Error(w, `{"error":"hash"}`, 500)
			return
		}
	}
	if err := s.Store.UpdateAdmin(admin.ID, username, hash); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			http.Error(w, `{"error":"username already taken"}`, 409)
			return
		}
		http.Error(w, `{"error":"update failed"}`, 500)
		return
	}
	if username == "" {
		username = admin.Username
	}
	writeJSON(w, 200, map[string]any{"ok": true, "username": username})
}

func (s *Server) issueCerts(w http.ResponseWriter, r *http.Request) {
	if s.IssueCerts == nil {
		http.Error(w, `{"error":"acme unavailable"}`, 500)
		return
	}
	if err := s.IssueCerts(); err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error(), "certs": s.certList()})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "certs": s.certList()})
}

func (s *Server) certList() []models.CertStatus {
	if s.Certs == nil {
		return nil
	}
	return s.Certs()
}

func (s *Server) kickCerts() {
	if s.IssueCerts == nil {
		return
	}
	go func() { _ = s.IssueCerts() }()
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.Store.Users()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, users)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username     string `json:"username"`
		Note         string `json:"note"`
		TrafficLimit int64  `json:"traffic_limit"`
		ExpireAt     string `json:"expire_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"bad json"}`, 400)
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" {
		http.Error(w, `{"error":"username required"}`, 400)
		return
	}
	var exp *time.Time
	if body.ExpireAt != "" {
		t, err := parseExpireDate(body.ExpireAt)
		if err != nil {
			http.Error(w, `{"error":"invalid expiry date"}`, 400)
			return
		}
		exp = t
	}
	u, err := s.Store.CreateUser(body.Username, body.Note, body.TrafficLimit, exp)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = s.Apply()
	writeJSON(w, 201, u)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	u, err := s.Store.UserByID(id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, 404)
		return
	}
	var body struct {
		Username      *string `json:"username"`
		Note          *string `json:"note"`
		Enable        *bool   `json:"enable"`
		TrafficLimit  *int64  `json:"traffic_limit"`
		ExpireAt      *string `json:"expire_at"`
		TelegramRegen *bool   `json:"telegram_regenerate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"bad json"}`, 400)
		return
	}
	if body.Username != nil {
		u.Username = *body.Username
	}
	if body.Note != nil {
		u.Note = *body.Note
	}
	if body.TrafficLimit != nil {
		u.TrafficLimit = *body.TrafficLimit
	}
	if body.ExpireAt != nil {
		t, err := parseExpireDate(*body.ExpireAt)
		if err != nil {
			http.Error(w, `{"error":"invalid expiry date"}`, 400)
			return
		}
		u.ExpireAt = t
	}
	if body.Enable != nil {
		u.Enable = *body.Enable
	} else if body.TrafficLimit != nil && !u.OverQuota() && !u.Expired() {
		// Raising/clearing a quota must bring the user back online. Previously
		// OverQuota permanently set enable=0, so extra GB left them off.
		u.Enable = true
	}
	if err := s.Store.UpdateUser(u); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if body.TelegramRegen != nil && *body.TelegramRegen {
		st, err := s.Store.Settings()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		domain := st.TelegramFakeDomain
		if domain == "" {
			domain = telegram.DefaultFakeDomain
		}
		if err := s.Store.SetTelegramSecret(id, crypto.TelegramFakeTLSSecret(domain)); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	_ = s.Apply()
	u, _ = s.Store.UserByID(id)
	writeJSON(w, 200, u)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err := s.Store.DeleteUser(id); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = s.Apply()
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) resetTraffic(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	_ = s.Store.ResetTraffic(id)
	_ = s.Apply()
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) userLinks(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	u, err := s.Store.UserByID(id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, 404)
		return
	}
	st, _, domains, inbounds, err := s.Store.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, map[string]any{
		"user":     u,
		"links":    core.UserLinks(u, st, domains, inbounds),
		"sub":      st.ClientPrefix() + "/" + u.SubToken,
		"clash":    st.ClientPrefix() + "/" + u.SubToken + "/clash",
		"sing_box": st.ClientPrefix() + "/" + u.SubToken + "/sing-box",
	})
}

func (s *Server) listDomains(w http.ResponseWriter, r *http.Request) {
	d, err := s.Store.Domains()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, d)
}

func (s *Server) createDomain(w http.ResponseWriter, r *http.Request) {
	var d models.Domain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, `{"error":"bad json"}`, 400)
		return
	}
	d.Enable = true
	if d.Mode == "" {
		d.Mode = models.ModeDirect
	}
	out, err := s.Store.CreateDomain(d)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = s.Apply()
	s.kickCerts()
	writeJSON(w, 201, out)
}

func (s *Server) updateDomain(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var d models.Domain
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		http.Error(w, `{"error":"bad json"}`, 400)
		return
	}
	d.ID = id
	if err := s.Store.UpdateDomain(d); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	_ = s.Apply()
	writeJSON(w, 200, d)
}

func (s *Server) deleteDomain(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	_ = s.Store.DeleteDomain(id)
	_ = s.Apply()
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) listInbounds(w http.ResponseWriter, r *http.Request) {
	in, err := s.Store.Inbounds()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, in)
}

func (s *Server) updateInbound(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(r.PathValue("id"), 10, 64)
	var body struct {
		Enable *bool `json:"enable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"bad json"}`, 400)
		return
	}
	if body.Enable != nil {
		if err := s.Store.SetInboundEnable(id, *body.Enable); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
	}
	_ = s.Apply()
	in, _ := s.Store.Inbounds()
	writeJSON(w, 200, in)
}

func (s *Server) previewConfig(w http.ResponseWriter, r *http.Request) {
	raw, err := s.compile()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	if err := s.Apply(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "core": s.Supervisor.Running()})
}

func (s *Server) compile() ([]byte, error) {
	st, users, _, inbounds, err := s.Store.Snapshot()
	if err != nil {
		return nil, err
	}
	return core.Compile(core.CompileInput{Settings: st, Users: users, Inbounds: inbounds, LogLevel: s.LogLevel})
}

func (s *Server) subUser(w http.ResponseWriter, r *http.Request) (models.User, models.Settings, []models.Domain, []models.Inbound, bool) {
	u, err := s.Store.UserByToken(r.PathValue("token"))
	if err != nil || !u.Active() {
		http.Error(w, "not found", 404)
		return models.User{}, models.Settings{}, nil, nil, false
	}
	st, _, domains, inbounds, err := s.Store.Snapshot()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return models.User{}, models.Settings{}, nil, nil, false
	}
	expire := int64(0)
	if u.ExpireAt != nil {
		expire = u.ExpireAt.Unix()
	}
	w.Header().Set("subscription-userinfo",
		"upload="+strconv.FormatInt(u.TrafficUp, 10)+
			"; download="+strconv.FormatInt(u.TrafficDown, 10)+
			"; total="+strconv.FormatInt(u.TrafficLimit, 10)+
			"; expire="+strconv.FormatInt(expire, 10))
	w.Header().Set("profile-update-interval", "1")
	w.Header().Set("content-disposition", `attachment; filename="soooski"`)
	return u, st, domains, inbounds, true
}

func (s *Server) subscription(w http.ResponseWriter, r *http.Request) {
	u, st, domains, inbounds, ok := s.subUser(w, r)
	if !ok {
		return
	}
	ua := strings.ToLower(r.Header.Get("User-Agent"))
	if isBrowser(ua) {
		s.writeClientPortal(w, r, u, st, domains, inbounds)
		return
	}
	switch {
	case strings.Contains(ua, "clash") || strings.Contains(ua, "stash") || strings.Contains(ua, "meta"):
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		_, _ = w.Write([]byte(core.ClashYAML(u, st, domains, inbounds)))
	case strings.Contains(ua, "sing-box") || strings.Contains(ua, "hiddify") || strings.Contains(ua, "sfavn"):
		raw, err := core.SingBoxClient(u, st, domains, inbounds)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	default:
		links := core.UserLinks(u, st, domains, inbounds)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(core.V2RaySubscription(links)))
	}
}

func (s *Server) subClash(w http.ResponseWriter, r *http.Request) {
	u, st, domains, inbounds, ok := s.subUser(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	_, _ = w.Write([]byte(core.ClashYAML(u, st, domains, inbounds)))
}

func (s *Server) subV2Ray(w http.ResponseWriter, r *http.Request) {
	u, st, domains, inbounds, ok := s.subUser(w, r)
	if !ok {
		return
	}
	links := core.UserLinks(u, st, domains, inbounds)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(core.V2RaySubscription(links)))
}

func (s *Server) subSingBox(w http.ResponseWriter, r *http.Request) {
	u, st, domains, inbounds, ok := s.subUser(w, r)
	if !ok {
		return
	}
	raw, err := core.SingBoxClient(u, st, domains, inbounds)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(raw)
}

func (s *Server) subInfo(w http.ResponseWriter, r *http.Request) {
	u, st, domains, inbounds, ok := s.subUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, 200, map[string]any{
		"username": u.Username,
		"enable":   u.Active(),
		"up":       u.TrafficUp,
		"down":     u.TrafficDown,
		"limit":    u.TrafficLimit,
		"expire":   u.ExpireAt,
		"links":    core.UserLinks(u, st, domains, inbounds),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
