package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/zwsq/soooski-panel/internal/config"
	"github.com/zwsq/soooski-panel/internal/store"
	"github.com/zwsq/soooski-panel/internal/supervisor"
)

func TestLoginUsersSub(t *testing.T) {
	cfg := config.Config{DataDir: t.TempDir(), AdminUser: "admin", AdminPass: "secret"}
	st, _, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	sup := supervisor.New("sing-box", cfg.SingBoxConfigPath(), "noop", nil)
	srv := &Server{Store: st, Supervisor: sup, Apply: func() error { return nil }, DataDir: cfg.DataDir}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Post(ts.URL+"/api/login", "application/json", bytes.NewBufferString(`{"username":"admin","password":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 401 {
		t.Fatalf("status %d", res.StatusCode)
	}
	_ = res.Body.Close()

	res, err = http.Post(ts.URL+"/api/login", "application/json", bytes.NewBufferString(`{"username":"admin","password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("login %d", res.StatusCode)
	}
	var login struct{ Token string }
	_ = json.NewDecoder(res.Body).Decode(&login)
	_ = res.Body.Close()
	if login.Token == "" {
		t.Fatal("no token")
	}

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/users", bytes.NewBufferString(`{"username":"carol"}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 201 {
		t.Fatalf("create %d", res.StatusCode)
	}
	var user map[string]any
	_ = json.NewDecoder(res.Body).Decode(&user)
	_ = res.Body.Close()
	token, _ := user["sub_token"].(string)
	if token == "" {
		t.Fatalf("user %#v", user)
	}

	res, err = http.Get(ts.URL + "/sub/" + token)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("sub %d", res.StatusCode)
	}
	_ = res.Body.Close()

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/sub/"+token, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) Chrome/120")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 200 || !bytes.Contains(body, []byte("soooski")) {
		t.Fatalf("portal %d %s", res.StatusCode, body)
	}

	hz, err := http.Get(ts.URL + "/healthz")
	if err != nil || hz.StatusCode != 200 {
		t.Fatalf("health %v %v", err, hz)
	}
	_ = hz.Body.Close()

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/admin", bytes.NewBufferString(`{"current_password":"nope","password":"newpass1","password_confirm":"newpass1"}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 401 {
		t.Fatalf("wrong current password %d", res.StatusCode)
	}
	_ = res.Body.Close()

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/admin", bytes.NewBufferString(`{"current_password":"secret","username":"root","password":"newpass1","password_confirm":"newpass1"}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("update admin %d %s", res.StatusCode, body)
	}
	_ = res.Body.Close()

	res, err = http.Post(ts.URL+"/api/login", "application/json", bytes.NewBufferString(`{"username":"root","password":"newpass1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("login after rename %d", res.StatusCode)
	}
	_ = res.Body.Close()
}

func TestQuotaReenableAndAdminPath(t *testing.T) {
	cfg := config.Config{DataDir: t.TempDir(), AdminUser: "admin", AdminPass: "secret"}
	st, _, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	sup := supervisor.New("sing-box", cfg.SingBoxConfigPath(), "noop", nil)
	srv := &Server{Store: st, Supervisor: sup, Apply: func() error { return nil }, DataDir: cfg.DataDir}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	res, err := http.Post(ts.URL+"/api/login", "application/json", bytes.NewBufferString(`{"username":"admin","password":"secret"}`))
	if err != nil {
		t.Fatal(err)
	}
	var login struct{ Token string }
	_ = json.NewDecoder(res.Body).Decode(&login)
	_ = res.Body.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/users", bytes.NewBufferString(`{"username":"dave","traffic_limit":100,"expire_at":"2099-01-02"}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 201 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("create %d %s", res.StatusCode, body)
	}
	var user struct {
		ID       int64  `json:"id"`
		ExpireAt string `json:"expire_at"`
	}
	_ = json.NewDecoder(res.Body).Decode(&user)
	_ = res.Body.Close()
	if user.ExpireAt == "" || !strings.HasPrefix(user.ExpireAt, "2099-01-02") {
		t.Fatalf("expiry should be end of 2099-01-02, got %q", user.ExpireAt)
	}

	if err := st.AddTraffic(user.ID, 80, 30); err != nil {
		t.Fatal(err)
	}
	if err := st.DisableUser(user.ID); err != nil {
		t.Fatal(err)
	}

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/users/"+strconv.FormatInt(user.ID, 10), bytes.NewBufferString(`{"traffic_limit":10737418240}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("update limit %d %s", res.StatusCode, body)
	}
	_ = res.Body.Close()
	got, err := st.UserByID(user.ID)
	if err != nil || !got.Enable || got.OverQuota() {
		t.Fatalf("raising the limit should re-enable: %#v %v", got, err)
	}

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/settings", bytes.NewBufferString(`{"admin_path":"my-panel/secretpath"}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("admin path %d %s", res.StatusCode, body)
	}
	var settings struct {
		AdminPath string `json:"admin_path"`
	}
	_ = json.NewDecoder(res.Body).Decode(&settings)
	_ = res.Body.Close()
	if settings.AdminPath != "my-panel/secretpath" {
		t.Fatalf("admin_path %q", settings.AdminPath)
	}

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/settings", bytes.NewBufferString(`{"telegram_enabled":"1","telegram_fake_domain":"www.cloudflare.com"}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("telegram on %d %s", res.StatusCode, body)
	}
	var tg struct {
		Enabled    bool   `json:"telegram_enabled"`
		FakeDomain string `json:"telegram_fake_domain"`
	}
	_ = json.NewDecoder(res.Body).Decode(&tg)
	_ = res.Body.Close()
	if !tg.Enabled || tg.FakeDomain != "www.cloudflare.com" {
		t.Fatalf("telegram settings %#v", tg)
	}

	dave, err := st.UserByID(user.ID)
	if err != nil || !strings.HasPrefix(dave.TelegramSecret, "ee") {
		t.Fatalf("per-user telegram secret %#v %v", dave, err)
	}
	firstSecret := dave.TelegramSecret

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/settings", bytes.NewBufferString(`{"telegram_enabled":"1","telegram_fake_domain":"www.cloudflare.com"}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	dave, _ = st.UserByID(user.ID)
	if dave.TelegramSecret != firstSecret {
		t.Fatal("secret should stay until domain change or regenerate")
	}

	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/users", bytes.NewBufferString(`{"username":"erin"}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 201 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("erin %d %s", res.StatusCode, body)
	}
	var erin struct {
		ID             int64  `json:"id"`
		TelegramSecret string `json:"telegram_secret"`
	}
	_ = json.NewDecoder(res.Body).Decode(&erin)
	_ = res.Body.Close()
	if erin.TelegramSecret == "" || erin.TelegramSecret == firstSecret {
		t.Fatalf("each user needs a distinct telegram secret: dave=%s erin=%s", firstSecret, erin.TelegramSecret)
	}

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/settings", bytes.NewBufferString(`{"telegram_enabled":"1","telegram_fake_domain":"www.google.com"}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	dave, _ = st.UserByID(user.ID)
	if dave.TelegramSecret == firstSecret || !strings.Contains(dave.TelegramSecret, "7777772e676f6f676c652e636f6d") {
		t.Fatalf("domain change should rewrite per-user secret: %s", dave.TelegramSecret)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/users/"+strconv.FormatInt(user.ID, 10)+"/links", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(raw), "tg://proxy") || !strings.Contains(string(raw), dave.TelegramSecret) {
		t.Fatalf("user links telegram %d %s", res.StatusCode, raw)
	}

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/users/"+strconv.FormatInt(user.ID, 10), bytes.NewBufferString(`{"telegram_regenerate":true}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("regen %d %s", res.StatusCode, body)
	}
	var rotated struct {
		TelegramSecret string `json:"telegram_secret"`
	}
	_ = json.NewDecoder(res.Body).Decode(&rotated)
	_ = res.Body.Close()
	if rotated.TelegramSecret == "" || rotated.TelegramSecret == dave.TelegramSecret {
		t.Fatalf("per-user regen %q vs %q", rotated.TelegramSecret, dave.TelegramSecret)
	}

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/settings", bytes.NewBufferString(`{"telegram_enabled":"1","telegram_fake_domain":"vpn.example.com"}`))
	req.Header.Set("Authorization", "Bearer "+login.Token)
	req.Header.Set("Content-Type", "application/json")
	stNow, _ := st.Settings()
	if stNow.PublicHost == "" {
		_ = st.SetSetting("public_host", "vpn.example.com")
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 400 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("own domain as fake tls should 400, got %d %s", res.StatusCode, body)
	}
	_ = res.Body.Close()
}

func TestParseExpireDate(t *testing.T) {
	got, err := parseExpireDate("2026-08-20")
	if err != nil || got == nil {
		t.Fatal(err, got)
	}
	if got.Hour() != 23 || got.Minute() != 59 {
		t.Fatalf("expected end of day, got %s", got)
	}
	empty, err := parseExpireDate("")
	if err != nil || empty != nil {
		t.Fatalf("empty: %v %v", empty, err)
	}
}
