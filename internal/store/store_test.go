package store

import (
	"strings"
	"testing"
	"time"

	"github.com/zwsq/soooski-panel/internal/config"
	"github.com/zwsq/soooski-panel/internal/crypto"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	cfg := config.Config{
		DataDir:   t.TempDir(),
		AdminUser: "admin",
		AdminPass: "secret",
	}
	s, pass, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if pass != "" {
		t.Fatal("password was provided, should not print generated one")
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUserCRUDAndInboundsSeeded(t *testing.T) {
	s := testStore(t)
	u, err := s.CreateUser("bob", "note", 1000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if u.UUID == "" || u.SubToken == "" || u.WGIP == "" || !strings.HasPrefix(u.TelegramSecret, "ee") {
		t.Fatalf("incomplete user %#v", u)
	}
	got, err := s.UserByToken(u.SubToken)
	if err != nil || got.Username != "bob" {
		t.Fatalf("by token: %v %#v", err, got)
	}
	u.Note = "x"
	if err := s.UpdateUser(u); err != nil {
		t.Fatal(err)
	}
	if err := s.AddTraffic(u.ID, 100, 50); err != nil {
		t.Fatal(err)
	}
	got, _ = s.UserByID(u.ID)
	if got.TrafficUp != 100 || got.TrafficDown != 50 {
		t.Fatalf("traffic %#v", got)
	}
	ins, err := s.Inbounds()
	if err != nil || len(ins) < 40 {
		t.Fatalf("inbounds %d %v", len(ins), err)
	}
	st, err := s.Settings()
	if st.RealityPublicKey == "" {
		t.Fatal(st, err)
	}
	if st.AdminPath == "" || !strings.Contains(st.AdminPath, "/") {
		t.Fatalf("admin path should be a hiddify-style secret, got %q", st.AdminPath)
	}
	if st.ClientPath == "" {
		t.Fatal("missing client path")
	}
	if st.TelegramEnabled {
		t.Fatal("telegram should be off by default")
	}
	if st.TelegramFakeDomain != "www.cloudflare.com" {
		t.Fatalf("telegram fake domain %q", st.TelegramFakeDomain)
	}
	exp := time.Now().Add(-time.Hour)
	u2, err := s.CreateUser("old", "", 0, &exp)
	if err != nil {
		t.Fatal(err)
	}
	if !u2.Expired() || u2.Active() {
		t.Fatal("expired user should be inactive")
	}
}

func TestInboundBackfillAndAdminNotReset(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Config{DataDir: dir, AdminUser: "admin", AdminPass: "secret"}
	s, _, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`DELETE FROM inbounds WHERE tag='vless-h2'`); err != nil {
		t.Fatal(err)
	}
	hash, err := crypto.HashPassword("changed8chars")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateAdmin(1, "", hash); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()

	cfg.AdminUser = "admin"
	cfg.AdminPass = "secret"
	s2, _, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	ins, err := s2.Inbounds()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, in := range ins {
		if in.Tag == "vless-h2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing inbound tags were not backfilled")
	}
	a, err := s2.AdminByUsername("admin")
	if err != nil {
		t.Fatalf("username must stay as first-boot value: %v", err)
	}
	if !crypto.CheckPassword(a.PasswordHash, "changed8chars") {
		t.Fatal("env must not overwrite a password set in the panel")
	}
}

func TestResetAdminClearsSessions(t *testing.T) {
	s := testStore(t)
	a, err := s.FirstAdmin()
	if err != nil {
		t.Fatal(err)
	}
	tok, err := s.CreateSession(a.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionAdmin(tok); err != nil {
		t.Fatal(err)
	}
	hash, err := crypto.HashPassword("brandnew9")
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ResetAdmin("root", hash)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "root" {
		t.Fatalf("username %q", got.Username)
	}
	if _, err := s.AdminByUsername("admin"); err == nil {
		t.Fatal("old username should be gone")
	}
	if _, err := s.SessionAdmin(tok); err == nil {
		t.Fatal("sessions must be invalid after reset")
	}
	again, err := s.AdminByUsername("root")
	if err != nil || !crypto.CheckPassword(again.PasswordHash, "brandnew9") {
		t.Fatal(again, err)
	}
}
